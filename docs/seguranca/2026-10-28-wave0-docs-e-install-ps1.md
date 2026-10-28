# Wave 0 — Threat Model: Documentação 11.x, Windows e install.ps1

> Data: 2026-10-28 | Roadmap: ROADMAP-2026-10-28-documentacao-e-canais-refletem-o-produto-11-x-com-suporte-real-a-windows-e-instalador-powershell.md
> Autor: hades-tf

---

## Seção 1 — Completude da Enumeração

### Comando executado

```
git grep -nE 'brew install|go install|pip install|npm install -g|irm |iex|LASTEXITCODE' \
  (saída completa nas subseções abaixo)
```

O grep foi executado em todo o repositório sem restrição de caminho.

### Superfícies user-facing (instruem o usuário ou um agente a instalar)

| Arquivo | Linha | Instrução | Estado |
|---|---|---|---|
| `README.md` | 96 | `brew install kgsaran/tap/trackfw` | ERRADO — tap real é `kgsaran/homebrew-trackfw` (`.goreleaser.yaml:59`). `kgsaran/homebrew-tap` → 404 confirmado |
| `README.md` | 102 | `go install github.com/kgsaran/trackfw/cmd/trackfw@latest` | Correto |
| `README.md` | 108, 146, 155, 302 | `npm install -g trackfw`, `pip install trackfw` | Corretos |
| `README.md` | 220–225 | Tabela de hook lines — forma D11 sem `emulate` | ERRADO: canonical atual em `internal/guardforms/guard_generations.go:23` tem `emulate -R sh 2>${null-/dev/null};` como prefixo (fix do #595, 10.1.0) |
| `docs/visao-projeto/VISION.md` | 204–205 | `brew install trackfw/tap/trackfw` e `go install github.com/trackfw/trackfw/cmd/trackfw@latest` | CRÍTICO — namespace `trackfw` não pertence ao projeto: `gh api users/trackfw` → 404, `gh api orgs/trackfw` → 404 |
| `docs/cli-parity.md` | 28 | `brew install trackfw` | Incompleto — falta o tap path |
| `docs/cli-parity.md` | 29–30 | `npm install -g trackfw`, `pip install trackfw` | Corretos |
| `docs/adr/ADR-2026-10-28-...` | 39–40 | `irm .../install.ps1 \| iex` | Correto (instrução da ADR sobre o novo instalador) |

### Instruções em arquivos de comandos de agente (.claude/, .gemini/)

```
git grep -l 'npm install -g trackfw\|pip install trackfw' .claude/commands/ .gemini/commands/
```

Resultado: múltiplos arquivos (`adr.md`, `move.md`, `req.md`, `status.md`, `validate.md`, `roadmap.md`) em `.claude/commands/trackfw/` e `.gemini/commands/trackfw/` exibem instruções `npm install -g trackfw` e `pip install trackfw`. Estas são superfícies user-facing (mostradas ao usuário/agente pelo CLI), mas **não são documentação de produto** — são slash-commands de contexto. São afetadas pela REQ, mas não pela seção de namespace: as instruções são corretas (npm e pip funcionam).

A linha `LASTEXITCODE` encontrada em `.claude/settings.json` e `.codex/hooks.json` é o hook instalado, não documentação de instalação — fora do escopo desta enumeração.

### Leitores mecânicos de `docs/cli-parity.md`

```
grep -rln 'cli-parity.md' internal scripts Makefile .github
```

Resultado (excluindo testdata):

**Scripts que consomem contratos anotados com `gate=` ou `partial=`:**
- `scripts/check-agent-namespace-union.sh` — lê seção `agent_models` e surfaces nomeadas
- `scripts/check-agent-models-parity.sh` — lê seção `agent_models — version composition`
- `scripts/check-barrier.sh` — lê `§four-pinned-exit-2-messages`, `§Pinned`, `§detection-is-a-full-pre-pass`
- `scripts/check-gates-falsify.sh` — lê `§detection-is-a-full-pre-pass`, `none` sections count
- `scripts/check-output-encoding-declared.sh` — lê anotação `partial=`
- `scripts/check-parity-contract-coverage.sh` — lê anotações `gate=` e `partial=` (arquivo principal: `DOC="${1:-$ROOT_DIR/docs/cli-parity.md}"`)
- `scripts/check-pr-closing-keyword.sh` — lê anotação `partial=`
- `scripts/check-ref-separator-portability.sh` — lê "Quatro propriedades exigidas de todo gate"

**Código Go:**
- `internal/commands/root.go` e derivados — usam `cli-parity.md` como referência de contrato (indireto, não parse)
- Vários arquivos Go referenciam o arquivo em comentários como autoridade de contrato

**Risco de ML-2B:** qualquer reescrita do `cli-parity.md` que mude identificadores de seção como `## \`agent_models\``, `## trackfw barrier`, `§four-pinned-exit-2-messages`, ou remova anotações `gate=` / `partial=` sem atualizar os scripts quebrará os gates correspondentes. A lista de strings fixadas consumidas pelos leitores mecânicos deve ser auditada antes da reescrita.

### Consumidores de `pypi/pyproject.toml`, `npm/README.md`, `pypi/README.md`

```
git ls-files npm/README.md pypi/README.md
```
Resultado: ambos os arquivos EXISTEM no repositório (rastreados pelo git).

**`pypi/README.md`:**
- Consumidor 1: `pypi/pyproject.toml` (`readme = { file = "README.md", content-type = "text/markdown" }`) — via `python -m build pypi/`, caminho setuptools. Este caminho NÃO é o caminho de build do CI.
- Consumidor 2: `.github/workflows/release.yml:309` — `cp README.md pypi/README.md` (escreve, não lê).
- `pypi/scripts/build_wheel.py` (o build real do CI): NÃO lê `pypi/README.md` em nenhum lugar; constrói `metadata_content` com campos fixos sem Description-Content-Type nem corpo (confirmado na inspeção das linhas 103–111 do script).
- **Veredito:** pode ser removido do versionamento (gerado em runtime pelo release.yml); remover quebra apenas o caminho setuptools que não é o caminho de produção. A remoção deve adicionar `pypi/README.md` ao `.gitignore`.

**`npm/README.md`:**
- `npm/package.json` `"files": ["bin/trackfw.js"]` — não lista README.md explicitamente, mas npm inclui README.md por padrão em todo pacote publicado.
- `.github/workflows/release.yml:237` (job `publish-npm-shim`, `defaults.run.working-directory: npm`): `cp ../README.md README.md` — **sobrescreve a cópia versionada** antes do publish. Medido: `sed -n '215,240p' .github/workflows/release.yml` → job `publish-npm-shim` + step `Copia README para o pacote npm` + `run: cp ../README.md README.md`.
- Nenhum gate, script ou código Go lê `npm/README.md` da árvore versionada. `grep -rn 'npm/README' scripts internal Makefile .github` → somente testdata de corpus (não é leitura de produção).
- **Veredito (CORRIGIDO):** a cópia versionada tem zero consumidores vivos — é sempre sobrescrita antes de ser usada. Pode ser removida do git e adicionada ao `.gitignore`. A página do npmjs continua recebendo descrição via root `README.md` copiado pelo release.yml.

**`pypi/pyproject.toml`:**
- Consumido pelo caminho setuptools e também por código Go e gates de produção.
- **Consumidores vivos (medido — `grep -rn 'pyproject' scripts internal Makefile .github` sem limit):**
  - `scripts/check-integration-assets.sh:81` — lê patterns de `package-data`
  - `scripts/check-manifest-version-gate.sh:153,155,159,161,163` — lê versão, falha se ausente
  - `scripts/check-release-tag-parity.sh:510,828` — **escreve** fixture sintética (`cat > … <<EOF`); não é leitura do arquivo real
  - `scripts/check-gates-falsify.sh:7663,7674` — copia para fixture de falsificação (read + copy)
  - `internal/commands/release.go:118` — `trackfw release` **lê** `pypi/pyproject.toml` para verificar versão (extrai com `extractPyprojectVersion`)
- **Veredito:** NÃO pode ser removido. O `trackfw release` (`internal/commands/release.go:118`) lê este arquivo para verificar e bumpar a versão do pacote PyPI. A remoção quebraria o fluxo de release além dos gates.

### Estado da lista

**A lista é fechada para esta REQ**, com ressalva: os arquivos de slash-commands (`.claude/commands/`, `.gemini/commands/`) têm instruções de instalação corretas e não precisam de correção de namespace. A seção de site (`site/`) e `CONTRIBUTING.md` estão na lista do roadmap e não foram enumeradas aqui em detalhe — o AC1 grep confirmará.

---

## Seção 2 — Modelo de Ameaça do `install.ps1`

O script não existe ainda. Esta análise modela o design descrito na ADR-2026-10-28.

### 2.1 — `irm | iex`: confiança transitiva na URL

**Vetor:** quem controla `https://github.com/kgsaran/trackfw/releases/latest/download/install.ps1` controla a execução. Vetores: compromisso do repositório GitHub, DNS hijack, MITM (HTTPS), GitHub CDN comprometida.

**O que o checksum NÃO protege:** o checksum SHA-256 protege o binário baixado pelo script, não o próprio script. Um script malicioso pode simplesmente pular a verificação.

**Mitigação documentada:** a ADR documenta a forma "baixe, leia, execute" como alternativa para quem não aceita pipe para `iex`. Esta é a única mitigação disponível dentro do próprio script.

**Residual aceito:** o projeto aceita este vetor como equivalente ao `curl | sh` já existente no `install.sh`, por decisão explícita na ADR. O vetor existe em ambos os instaladores.

### 2.2 — Checksum: quatro estados de ataque

A. **Ausente** (`FILENAME` não encontrado em `checksums.txt`): o `install.sh` aborta com mensagem "ausente". O `install.ps1` deve replicar este comportamento. Contra-braço: o arquivo de destino NÃO pode existir após abort — afirmar por presença/ausência do arquivo, não apenas pelo exit code.

B. **Duplicado** (`FILENAME` aparece mais de uma vez): o `install.sh` aborta com mensagem "duplicado". Mesma exigência de afirmação por ausência do arquivo.

C. **Divergente** (hash calculado ≠ hash publicado): aborta com mensagem "divergente". O tarball pode ter sido adulterado em trânsito ou substituído.

D. **`checksums.txt` de outra tag** (downgrade attack): o adversário serve o `checksums.txt` de `v10.0.0` com o tarball de `v10.0.0`. O FILENAME inclui a versão (`trackfw_11.1.0_windows_amd64.tar.gz`), logo o checksum da versão errada não casaria com o FILENAME correto — **o vetor é bloqueado por construção**, porque o FILENAME contém VERSION_BARE. Porém: se o adversário servir simultaneamente o checksums.txt e o tarball de uma versão mais antiga, ambos com hashes corretos entre si, o checksum PASSA mas a versão instalada é a mais antiga. **Mitigação necessária no gate:** verificar que o binário instalado reporta a versão esperada (`trackfw.exe --version` após install).

### 2.3 — Injeção em `TRACKFW_VERSION` e `TRACKFW_ARCH`

**Vetor central:** o usuário ou o ambiente de CI define estas variáveis. Um valor malicioso pode escapar da validação e: (a) compor uma URL ou caminho de arquivo com `..` ou separadores que escrevam fora de `$TMP_DIR`; (b) executar comandos via separadores de shell.

**`TRACKFW_VERSION`:**

O `install.sh` usa `case` (âncora no buffer inteiro). O equivalente em PowerShell DEVE usar regex com `\A` e `\z` em vez de `^` e `$`:

```powershell
# ERRADO 1: -match e case-insensitive; $ casa antes de \n final; \d casa digitos Unicode
if ($ver -match '^v?\d+\.\d+\.\d+(-rc\d+|-beta\d+)?$') { ... }
# CORRETO: -cmatch (case-sensitive); \A e \z; [0-9] ASCII; sufixo 1-3 digitos como install.sh
if ($ver -cmatch '\Av?[0-9]+\.[0-9]+\.[0-9]+(-rc[0-9]{1,3}|-beta[0-9]{1,3})?\z') { ... }
```

Em PowerShell, `$` em modo padrão casa antes de um `\n` **final** do string. Medido (`regex_test.ps1`):

```
"v7.3.0"+LF      matches ^v7\.3\.0$         : True   ← vetor real (trailing newline passa)
"v7.3.0"+LF+FOO  matches ^v7\.3\.0$         : False  ← (versao anterior dizia True — ERRADO)
"v7.3.0"+LF      matches \Av7\.3\.0\z       : False  ← \z bloqueia o trailing newline
"v7.3.0"         matches \Av7\.3\.0\z       : True
```

O vetor real é `"v7.3.0" + [char]10` (trailing newline) — não `"v7.3.0`n`FOO"`. Um valor com newline final passa a validação `^...$` e a versão com newline embutido é extraída para URL/FILENAME.

**Dígitos Unicode:** `-match '\A\d\z'` → True para `[char]0x0663` (dígito árabe-índico). `-match '\A[0-9]\z'` → False. Usar `[0-9]` em vez de `\d`.

**Case sensitivity:** `-match` é case-insensitive: `"V11.1.0" -match '\Av?\d+\.\d+\.\d+\z'` → True. `-cmatch` é case-sensitive: `"V11.1.0" -cmatch '\Av?[0-9]+\.[0-9]+\.[0-9]+\z'` → False. O `install.sh` aceita somente `v` minúsculo (lido: `case "$_tv_raw" in v*)`). O `install.ps1` deve espelhar: `-cmatch '\Av?[0-9]+...` — sem `[vV]?`.

**Alvo real de um path traversal:** não a URL remota (o GitHub normaliza o path do lado do servidor), mas o argumento de `-OutFile` ou `Expand-Archive -DestinationPath`. Se `VERSION_BARE` contivesse `/` ou `\`, o `Join-Path $TMP_DIR $FILENAME` poderia escrever fora de `$TMP_DIR`. A regex com `\A...\z` que aceita apenas dígitos, `.` e sufixos `-rc`/`-beta` bloqueia este vetor.

**Vetores testados obrigatórios no gate (espelhando `check-install-version-pin.sh`):**
- `7.3.0; Remove-Item -Recurse -Force $env:TEMP` — `;` é separador PS
- `$(Get-Date)` — subexpressão PS
- `v7.3.0` + newline (`"v7.3.0" + [char]10`) — trailing newline como vetor real de âncora
- `../../etc/passwd` — path traversal
- `7.3.0/../../tmp/evil` — path traversal com prefixo válido

**`TRACKFW_ARCH`:**
Deve ser validado por lista fechada: `amd64|arm64`. Qualquer outro valor → abort. Sem regex, sem substring match — apenas `if ($arch -ne 'amd64' -and $arch -ne 'arm64') { abort }`.

### 2.4 — `TRACKFW_INSTALL_DIR` com junction ou symlink

**Vetor:** `$env:TRACKFW_INSTALL_DIR = "C:\evil_junction"` onde `C:\evil_junction` é uma junction que aponta para um diretório do sistema. O binário instalado em `C:\evil_junction\trackfw.exe` sobrescreve um arquivo no destino real da junction.

**Estado da ADR:** o destino padrão é `$env:LOCALAPPDATA\Programs\trackfw` (por-usuário, sem elevação). A ADR NÃO exige verificação explícita de junction/symlink no `TRACKFW_INSTALL_DIR`.

**Avaliação:** o risco é baixo para o destino padrão (o usuário controla seu próprio `%LOCALAPPDATA%`). Para `TRACKFW_INSTALL_DIR` arbitrário, o instalador deve verificar se o diretório destino é uma junction (`(Get-Item $dir -ErrorAction SilentlyContinue).Attributes -band [IO.FileAttributes]::ReparsePoint`). Se for junction, emitir aviso mas não abortar — o comportamento é consistente com o `install.sh` que não verifica junctions.

**Nota de vault relevante:** `winsymlink-gomod-governa-mode-de-juncao-2026-09-27.md` — junctions no Windows com `go.mod >= 1.23` aparecem como `ModeIrregular`, não `ModeSymlink`. Não afeta o `install.ps1` diretamente (PowerShell usa `FileAttributes.ReparsePoint`).

### 2.5 — Corrida em `%TEMP%`

**Vetor:** entre a escolha do nome de `$TMP_DIR` e a criação do diretório, outro processo cria um symlink com o mesmo nome apontando para um destino controlado pelo atacante.

**Mitigação prescrita:**
```powershell
$TMP_DIR = Join-Path ([System.IO.Path]::GetTempPath()) ([System.IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Path $TMP_DIR -ErrorAction Stop | Out-Null
# New-Item falha (terminating error) se o path já existir — não usar -Force aqui
```

`GetRandomFileName()` gera 11 caracteres aleatórios (base-32), produzindo ~57 bits de entropia. A janela de tempo entre `GetRandomFileName()` e `New-Item` é de microssegundos. Ataque impraticável em ambiente normal. `-ErrorAction Stop` (não `-Force`) garante falha explícita se o path já existir (proteção contra symlink plantado).

### 2.6 — Extração por `tar.exe` com membros `..` ou absolutos

**Estado medido:** o `tar.exe` nativo do Windows 10 1803+ é o bsdtar (libarchive). Por padrão, bsdtar **descarta** `..` em paths de membros e strips paths absolutos (e.g., `C:\evil\trackfw.exe` → `evil\trackfw.exe`). Este comportamento está documentado no código-fonte do libarchive e é consistente com o comportamento do tar GNU.

**Não verificado:** o comportamento exato não foi medido em CI para confirmar que a versão específica de `tar.exe` no runner `windows-latest` do GitHub Actions mantém esta proteção. O gate de ML-1A deve incluir um cenário que tenta extrair um tarball com um membro `../evil` e confirma que o arquivo NÃO aparece fora de `$TMP_DIR`.

**Mitigação adicional recomendada:** após extração, verificar que o binário instalado está dentro do `$TMP_DIR` esperado antes de movê-lo para `$INSTALL_DIR`. Pattern: `$extracted = Join-Path $TMP_DIR "trackfw.exe"; if (-not (Test-Path $extracted)) { abort }`.

### 2.7 — Alteração do PATH do usuário: REG_EXPAND_SZ, duplicação, entrada vazia

**REG_EXPAND_SZ (bug documentado do .NET):**

`[Environment]::SetEnvironmentVariable("PATH", $newPath, [System.EnvironmentVariableTarget]::User)` chama a Win32 API `SetEnvironmentVariable`, que escreve `REG_SZ` em `HKCU\Environment\PATH`, independentemente do tipo original.

Se o PATH existente for `REG_EXPAND_SZ` (contém entradas como `%USERPROFILE%\AppData\Local\Microsoft\WindowsApps`), regravá-lo como `REG_SZ` faz essas variáveis deixarem de ser expandidas — o PATH do usuário fica literalmente com `%USERPROFILE%` como texto, e essas entradas param de funcionar.

**Mitigação prescrita (única correta):**
```powershell
$regKey = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment', $true)
$existingType = $regKey.GetValueKind('PATH')
if ($existingType -eq [Microsoft.Win32.RegistryValueKind]::ExpandString) {
    $regKey.SetValue('PATH', $newPath, [Microsoft.Win32.RegistryValueKind]::ExpandString)
} else {
    $regKey.SetValue('PATH', $newPath, [Microsoft.Win32.RegistryValueKind]::String)
}
$regKey.Close()
```

Alternativamente: `Set-ItemProperty -Path "HKCU:\Environment" -Name "PATH" -Value $newPath -Type ExpandString` sempre grava `REG_EXPAND_SZ`, o que é seguro mesmo quando o original era `REG_SZ` (upgrade silencioso e inócuo).

A ADR exige explicitamente "preservando REG_EXPAND_SZ". Este é o único caminho correto.

**Duplicação:** o verificador de idempotência deve usar split por `;` e comparar caminhos normalizados (trailing `\` removida, case-insensitive). Não pode usar `Contains()` simples — `C:\foo` e `C:\foo\` seriam tratados como diferentes.

**Entrada vazia:** ao fazer `$newPath = $oldPath + ";" + $INSTALL_DIR`, se `$oldPath` estiver vazio, o resultado é `;C:\...`, inserindo uma entrada vazia no início. Verificar `if ($oldPath -ne '') { $newPath = $oldPath + ";" + $INSTALL_DIR } else { $newPath = $INSTALL_DIR }`.

**`TRACKFW_NO_MODIFY_PATH=1`:** quando definido, o instalador não toca o PATH. O gate deve verificar que o PATH do usuário não foi alterado após instalação com esta flag.

### 2.8 — Processo x64 emulado em ARM64 (`PROCESSOR_ARCHITECTURE` vs `PROCESSOR_ARCHITEW6432`)

**Problema:** em Windows ARM64, o PowerShell 5.1 é um processo x64 rodando emulado. Nesse caso:
- `$env:PROCESSOR_ARCHITECTURE` = `AMD64` (reporta o processo emulado, não o host)
- `$env:PROCESSOR_ARCHITEW6432` = pode ser `ARM64` (variável WoW64 para indicar arquitetura nativa) — presente em processos 32-bit/WoW64, **não garantido** em processos 64-bit emulados

**Comportamento medido (VM ARM64, 2026-10-04, docs/portabilidade/2026-10-04-trackfw-no-path-dos-shells-do-windows-por-canal.md):** instalação via npm com PowerShell confirmou ARM64 na VM. O comportamento de `PROCESSOR_ARCHITEW6432` em PS 5.1 especificamente não está medido.

**PowerShell 7 (arm64 nativo):**
- `$env:PROCESSOR_ARCHITECTURE` = `ARM64` — confiável
- `[System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture` disponível (.NET 5+)

**PowerShell 5.1 (x64 emulado em ARM64):**
- `[System.Runtime.InteropServices.RuntimeInformation]` NÃO está disponível (requer .NET Core/5+)
- A abordagem correta: verificar `$env:PROCESSOR_ARCHITECTURE` primeiro; se for `AMD64`, verificar `$env:PROCESSOR_ARCHITEW6432`; se este for `ARM64`, selecionar `arm64`. Se `PROCESSOR_ARCHITEW6432` não estiver definido, usar `$env:PROCESSOR_ARCHITECTURE` normalmente.

```powershell
$arch = if ($PSVersionTable.PSVersion.Major -ge 7) {
    [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLower()
} elseif ($env:PROCESSOR_ARCHITEW6432 -eq 'ARM64') {
    'arm64'
} elseif ($env:PROCESSOR_ARCHITECTURE -eq 'AMD64') {
    'amd64'
} elseif ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') {
    'arm64'
} else {
    $null  # arquitetura não reconhecida → abort
}
```

**Não verificado:** comportamento de `PROCESSOR_ARCHITEW6432` em processos PS 5.1 de 64-bit. A documentação Microsoft indica que esta variável é definida para processos WoW64 (32-bit rodando em 64-bit), não necessariamente para 64-bit emulado em ARM64. A medição em VM ARM64 com PS 5.1 (AC9) deve confirmar qual variável está disponível.

### 2.9 — Decisão: mecanismo de injeção de fixture no gate

**Restrição do arquiteto:** o `install.ps1` de produção **não pode conter** nenhuma variável de ambiente ou parâmetro que redirecione downloads (sem `TRACKFW_BASE_URL`, sem `TRACKFW_MIRROR`, sem `TRACKFW_INSTALL_DRYRUN`). Motivo: a mesma URL serve o tarball **e** o `checksums.txt`. Quem define `$env:TRACKFW_BASE_URL` derrota a verificação de integridade por inteiro — checksum e tarball vêm do atacante. `$env:TRACKFW_INSTALL_DRYRUN` é a mesma classe: comportamento toggleado por estado ambiente inerte.

**Precedente (`install.sh`):** `scripts/check-install-checksum.sh` linhas 46-48, 199-284 — o gate prepende `$STUB_BIN` ao `PATH` e o `install.sh` chama `curl`/`wget` pelo nome simples. O script de produção não tem porta de redirecionamento. Medido: `grep -nE 'BASE_URL|DOWNLOAD' scripts/install.sh` → vazio.

**Mecanismo PowerShell análogo: shadowing por função no escopo chamador**

Em PowerShell, uma função definida no escopo do script de gate é visível nos escopos filhos quando `& install.ps1` é invocado. O script filho chama `Invoke-WebRequest` pelo nome não-qualificado; o lookup de escopo encontra a função do pai antes do cmdlet do módulo. O gate grava cada chamada em um log; o `install.ps1` não escreve nenhum flag — somente o shadow escreve.

**Medição (arms A-E, harness `gate2.ps1`):**

```
Arm A: shadow definido, call nao-qualificada, versao valida
  [A_shadow_unqualified] log_lines=1
    SHADOW:http://127.0.0.1:9/trackfw_v11.1.0_windows_amd64.tar.gz
  ← somente o shadow escreve no log; nenhuma rede real

Arm B: SEM shadow (gate_no_shadow.ps1), call nao-qualificada
  [B_no_shadow] log_lines=0
  stderr: Connection refused (cmdlet real chamado, porta fechada)
  ← contra-braco: sem shadow, o cmdlet real e atingido

Arm C: shadow definido, call qualificada Microsoft.PowerShell.Utility\Invoke-WebRequest
  [C_qualified_bypass] log_lines=0
  stderr: Connection refused (cmdlet real chamado, shadow ignorado)
  ← bypass confirmado: install.ps1 NAO DEVE usar o nome qualificado

Arm C1: shadow serve fixture por basename da URL (gate_c1v2.ps1, 2 calls)
  [gate] fixture hash: 8180c7991ee95572ee456c1d149e6dc3b10c0a8598677110241e1542b1c4f234
  [shadow] served: trackfw_v11.1.0_windows_amd64.tar.gz → $TMPDIR/dl_tar.out
  [shadow] served: checksums.txt → $TMPDIR/checksums.out
  child result: TARBALL_HASH:8180C7991EE95572EE456C1D149E6DC3B10C0A8598677110241E1542B1C4F234
  [C1_fixture_proper] log_lines=2
    SHADOW:http://127.0.0.1:9/trackfw_v11.1.0_windows_amd64.tar.gz
    SHADOW:http://127.0.0.1:9/checksums.txt
  ← shadow copia fixture via [IO.File]::Copy (funciona em PS 5.1 e 7); hash bate

Arm D: versao invalida ("v7.3.0; Remove-Item -Recurse -Force C:\")
  [D_invalid_version] exit=2 log_lines=0   ← rejeitado antes do download; zero calls

Arm E: versao valida (nao-vacuidade de D)
  [E_valid_version_nonvacuity] log_lines=1   ← prova que D nao e vacuo
```

**Por que shadowing não é estado ambiente do mesmo tipo que BASE_URL:**
- `BASE_URL` é dado inerte (string) que chega de fontes sem execução de código (CI env, `.envrc`, docker). O atacante só precisa persistir uma variável.
- O shadow requer **execução de código** no escopo chamador — já é compromisso total da sessão. Mesmo bar que o stub no PATH do `install.sh`.
- O shadow não abre nenhuma seam no `install.ps1`. É uma propriedade de lookup de escopo que existe independentemente.

**Modo `irm | iex`:** quando o script é executado via `Invoke-Expression`, roda no escopo do chamador — o shadow também se aplica. Medido (`test_irm_iex2.ps1`): `Shadow hit: True, Script continued: True`. Cuidado: `exit N` dentro de um scriptblock executado por `Invoke-Expression` **termina o processo chamador** (medido: `pwsh -Command "Invoke-Expression 'exit 3'; Write-Host 'AFTER'"` → exitcode=3, "AFTER" não impresso). O arm `irm|iex` no gate deve rodar em processo filho — e o shadow deve ser definido dentro do bloco filho: `pwsh -Command "function Invoke-WebRequest {...}; Invoke-Expression (Get-Content install.ps1 -Raw)"`.

**PS 5.1:** não verificado aqui — `pwsh` usado é PS 7.x. O CI `windows-latest` roda os dois (`powershell` = 5.1, `pwsh` = 7.x). O comportamento de shadowing deve ser verificado sob `powershell` (5.1) no CI — não foi medido nesta sessão.

**Decisão final: função shadow `Invoke-WebRequest` no escopo do gate.**

Prescrições mandatórias para `install.ps1`:
1. Chamar `Invoke-WebRequest` **sem qualificação de módulo** (nunca `Microsoft.PowerShell.Utility\Invoke-WebRequest`).
2. Não conter: `TRACKFW_BASE_URL`, `TRACKFW_MIRROR`, `TRACKFW_INSTALL_DRYRUN`, `Invoke-RestMethod`, `irm` (standalone), `curl.exe`, `wget.exe`, `WebClient`, `HttpClient`, `BitsTransfer`.

**Verificação estática obrigatória no gate** (`check-install-ps1.ps1`, executada antes de qualquer run):
```powershell
$banned = 'TRACKFW_BASE_URL|TRACKFW_MIRROR|TRACKFW_INSTALL_DRYRUN|Microsoft\.PowerShell\.Utility\\|WebClient|HttpClient|BitsTransfer|Invoke-RestMethod|\birm\b|curl\.exe|wget\.exe'
if (Select-String -Path $INSTALL_PS1 -Pattern $banned -Quiet) {
    Write-Error "FAIL: install.ps1 contem construcao proibida"; exit 2
}
```
Nota sobre aliases (medido): `iwr` é alias de `Invoke-WebRequest` e **é interceptado pelo shadow** — chamando `iwr` no filho, o shadow registrou `SHADOW:...`. `irm` é alias de `Invoke-RestMethod` (cmdlet diferente, não interceptado). Por isso `iwr` não precisa estar na lista de banidos por segurança, mas `\birm\b` deve estar. Também medido: `\birm\b` **não** casa "confirm" (False); o padrão sem o `\b` inicial (`irm\b`) casaria (True em "confirm") — por isso o ban list usa `\b` em ambas as extremidades.

**Armadilha de criação de fixture (medido):** `Set-Content -Encoding Byte` falha em PS 7 (`Byte is not a supported encoding name`). Usar `[System.IO.File]::WriteAllBytes(path, bytes)` funciona em PS 5.1 e 7.

**Armadilha de implementação do shadow (medido):** a função shadow com `[CmdletBinding()]` já recebe os common parameters (`-ErrorAction`, `-Verbose`, etc.) automaticamente. Re-declarar `-ErrorAction` em `param()` causa `"A parameter with the name 'ErrorAction' was defined multiple times"`. O shadow deve declarar apenas os parâmetros **não-common** que o `install.ps1` passa explicitamente: `-Uri`, `-OutFile`, `-UseBasicParsing`.

O gate afirma:
- log de calls tem exatamente 2 URLs (tarball + checksums.txt) da tag correta (espelhando AC1 "da mesma tag");
- zero calls quando a versão é inválida (arm D); versão válida produz calls (arm E — não-vacuidade).

**Cenários de checksum (C1-C5):** exercitados via fixture local servida pelo shadow (grava o conteúdo de `$OutFile`), sem `HttpListener` e sem variável de redirecionamento.

---

## Seção 3 — Namespace

### Medição — histórico e atualização

**Antes de 2026-10-28T21:27:34Z** (bounded pelo `created_at` da org, medido abaixo):
```
gh api users/trackfw → {"message":"Not Found","status":"404"}
gh api orgs/trackfw  → {"message":"Not Found","status":"404"}
```
A janela de hijack estava aberta: qualquer pessoa podia registrar o namespace `trackfw` e servir fórmulas/módulos maliciosos via as instruções erradas do VISION.md.

**Medição atual (2026-10-28):**
```
gh api orgs/trackfw --jq '"\(.login) \(.type) \(.created_at)"'
→ trackfw Organization 2026-10-28T21:27:34Z

gh api orgs/trackfw --jq .two_factor_requirement_enabled
→ true

gh api repos/trackfw/homebrew-tap → {"message":"Not Found","status":"404"}
gh api repos/trackfw/trackfw      → {"message":"Not Found","status":"404"}
gh api repos/kgsaran/homebrew-tap → {"message":"Not Found","status":"404"}
```

### Análise

**`trackfw` (namespace GitHub):**

O namespace foi registrado pelo KG em 2026-10-28T21:27:34Z como organização, com 2FA obrigatório habilitado. O vetor de hijack está **fechado**: nenhum atacante pode registrar `trackfw` no GitHub.

Os repositórios `trackfw/homebrew-tap` e `trackfw/trackfw` ainda não existem. Enquanto não criados, um atacante que comprometesse a organização poderia criá-los, mas o vetor exige comprometer a org — que agora pertence ao KG. Severidade reduzida de CRÍTICO para BAIXO.

**Residual pós-registro:**
- `docs/visao-projeto/VISION.md:204-205` ainda referencia `brew install trackfw/tap/trackfw` e `go install github.com/trackfw/trackfw/...`. Usuários que tentarem agora receberão **404** (repositórios não existem), não código malicioso.
- Para `go install`: o proxy Go também não tem o módulo cacheado. Medido: `curl -s https://proxy.golang.org/github.com/trackfw/trackfw/@v/list` → `"not found: module github.com/trackfw/trackfw"`. Não há versão pré-cacheada do período em que a janela esteve aberta. O dano é falha de instalação.
- A correção das instruções (ML-3A, ML-3B) ainda é necessária para remover o bug funcional.

**`kgsaran/homebrew-tap` (instrução errada no README):**

`README.md:96` diz `brew install kgsaran/tap/trackfw`. A notação `kgsaran/tap` expande para `github.com/kgsaran/homebrew-tap`, que não existe (404 confirmado). O tap real é `kgsaran/homebrew-trackfw` (`.goreleaser.yaml:59`). Instrução correta:
```bash
brew install kgsaran/trackfw/trackfw
```
Este é um bug funcional. O risco de segurança é menor: criar `kgsaran/homebrew-tap` requer comprometer a conta `kgsaran`, não registrar um novo usuário. O vetor original era registrar `trackfw` — isso está fechado.

### Ações recomendadas

1. **(Fora do repositório — CONCLUÍDO):** organização `trackfw` registrada com 2FA. Vetor de supply chain via namespace fechado.
2. **(Dentro do repositório, ML-3B e ML-3A):** substituir todas as referências a `trackfw/tap`, `github.com/trackfw/trackfw` e `kgsaran/tap` pelos namespaces corretos.
3. **(Verificação do AC1):** o gate `git grep -nE 'github\.com/trackfw/|trackfw/tap|kgsaran/tap' -- README.md CONTRIBUTING.md docs/visao-projeto ...` deve retornar vazio após os MLs.

---

## Seção 4 — Alvos de Falsificação nas Duas Direções

### Superfície A: linha de hook no README (AC2)

**Estado atual:** `README.md:220-225` exibe a forma D11 sem `emulate` (`$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard <name>; ...`). A forma canônica atual em `internal/guardforms/guard_generations.go:23` tem `emulate -R sh 2>${null-/dev/null};` como prefixo.

O teste `readme_line_test.go` (novo, ML-3A) deve importar a constante diretamente do pacote `guardforms`, não hardcodá-la.

**Falso Negativo (FN — gate não detecta divergência):**
- Cenário: README é revertido à forma D11 sem `emulate` (como está hoje). Gate deve reprovar.
- Sabotagem de verificação: `readme_line_test.go` usa a string hardcoded da forma antiga em vez de importar `guardforms.CredentialCurrentPSPOSIX`. Gate passa com README errado.
- **Contra-braço obrigatório:** o teste DEVE importar `guardforms.CredentialCurrentPSPOSIX` diretamente; se a constante mudar, o teste muda sem intervenção.

**Falso Positivo (FP — gate rejeita linha válida):**
- Cenário: `guardforms.CredentialCurrentPSPOSIX` é atualizado para uma nova forma (e.g., D12) antes do README ser atualizado. Gate reprova o README com a forma antiga, que ainda está documentada.
- Este é o comportamento DESEJADO: o gate deve reprovar até o README ser atualizado. Não é um FP a evitar — é a detecção correta.
- FP genuíno: se o gate extrair a linha do README de forma que inclua espaços ou formatação de markdown que não fazem parte da string literal. A extração deve usar a string exata dentro do bloco de código, sem markup.

### Superfície B: METADATA da wheel PyPI (AC3)

**Estado atual:** `build_wheel.py` gera METADATA sem `Description-Content-Type` e sem corpo. A página do PyPI 11.1.0 tem 0 caracteres de descrição.

O gate `check-wheel-metadata-readme.sh` (novo, ML-1B) deve construir uma wheel e inspecionar o METADATA.

**FN (gate não detecta ausência):**
- Cenário A: `Description-Content-Type` removido do METADATA → gate deve reprovar.
- Sabotagem: gate verifica apenas a presença da string `Description-Content-Type` mas não verifica se o corpo após os campos de cabeçalho é não-vazio → gate passa com tipo presente mas corpo vazio.
- **Contra-braço:** o gate deve verificar (1) presença de `Description-Content-Type: text/markdown`, (2) corpo do METADATA não vazio (pelo menos um byte após a linha em branco que separa cabeçalhos do corpo), (3) conteúdo igual ao README (ou subconjunto verificável).

**FP (gate rejeita wheel válida):**
- Cenário: `build_wheel.py` grava `Description-Content-Type: text/x-rst` em vez de `text/markdown` → gate deve reprovar (tipo errado, não é FP).
- FP genuíno: gate falha porque o README contém caracteres que o grep ou comparação de string do gate não interpreta corretamente (e.g., caracteres UTF-8 multi-byte). A solução é usar comparação binária ou Python em vez de grep para este check.

### Superfície C: gate do `install.ps1` — cenários de checksum

**Para cada cenário, ambas as direções devem ser testadas:**

| Cenário | FN: gate não detecta falha | FP: gate rejeita sucesso | Contra-braço |
|---|---|---|---|
| C1 íntegro → instala | Script falha ao instalar arquivo correto; gate não detecta ausência do binário | Gate verifica só o exit code 0, não a presença do binário | Afirmar que `$INSTALL_DIR\trackfw.exe` existe E tem hash correto após instalação íntegra |
| C2 adulterado → sem binário | Script instala com hash errado (não verifica); gate não detecta presença indevida | Gate verifica só o exit code ≠ 0, não a ausência do arquivo | Afirmar que `$INSTALL_DIR\trackfw.exe` NÃO existe após instalação adulterada |
| C3 ausente → abort | Script passa sem verificar; gate não detecta | — | Afirmar ausência do binário E presença de mensagem "ausente" no stderr |
| C4 duplicado → abort | Script aceita primeiro hash encontrado; gate não detecta | — | Afirmar ausência do binário E presença de mensagem "duplicado" no stderr |
| C5 hash errado → abort | Script passa com hash errado; gate não detecta | — | Afirmar ausência do binário E presença de mensagem "divergente" no stderr |

**Não-vacuidade obrigatória:** para cada cenário C2-C5, o contra-braço de não-vacuidade (sabotagem: remover a verificação do `install.ps1` e confirmar que o gate falha) deve ser executado e revertido durante o desenvolvimento, com resultado documentado no relatório do ML-1A.

### Superfície C2: mecanismo shadow do gate (ML-1A)

**FN (bypass não detectado):** install.ps1 usa `Microsoft.PowerShell.Utility\Invoke-WebRequest` (nome qualificado) — o shadow não intercepta, o gate não registra calls, C2-C5 aparentam green porque o script abortou por outro motivo (ex.: rede indisponível). Contra-braço: o gate deve verificar que o log tem **exatamente** 2 entradas com URLs sob a tag correta em todo arm que alcança o download.

**FP (gate rejeita instalação válida):** shadow mal implementado (ex.: re-declara `ErrorAction`, log não é limpo entre arms). Contra-braço: arm C1 (fixture íntegra) deve produzir log_lines=2 e binário com hash correto.

### Superfície C3: lista de banidos estática (ML-1A)

**FN (primitiva não coberta):** install.ps1 usa `Invoke-RestMethod` (não interceptado) ou `curl.exe` (externo). Contra-braço: plantar token banido em cópia temporária do `install.ps1` e verificar que o grep estático falha.

**FP (ban list rejeita construção legítima):** o padrão `\birm\b` sem o `\b` inicial causaria FP em "confirm" (medido: `'confirm' -match 'irm\b'` → True). Com `\birm\b` não há FP: `'confirm' -match '\birm\b'` → False; `'firm' -match '\birm\b'` → False. O ban list atual usa `\birm\b` corretamente.

### Superfície D: linha de hook PS/POSIX no gate (AC5, ML-2A)

**Estado atual:** `README.md:220-225` mostra a forma D11 sem `emulate`. O hook instalado em `.claude/settings.json` e `.codex/hooks.json` usa a forma com `emulate`. O teste `TestGuardLineShellMatrix` (existente) cobre bash/sh.

**Extensão para `pwsh` e `powershell` (ML-2A):**

**FN (guard emite forma errada, gate não detecta):**
- Cenário: `guard init` emite a forma sem `emulate` no hook de `pwsh`. O usuário do zsh que copia a linha documentada no README (que também está sem `emulate`) executa o hook sem proteção zsh — fail-open sob zsh confirmado no #595.
- **Contra-braço:** o teste deve verificar que a forma gerada pelo `trackfw init` para o hook de Claude Code contém `emulate -R sh` quando executado por `pwsh`.

**FP (gate rejeita forma válida):**
- Cenário: um CLI específico (e.g., Kiro) usa `|| exit 2` sem `emulate`. O gate verifica a forma errada para este CLI.
- **Contra-braço:** o teste deve verificar a forma correta POR CLI, não uma forma única.

**Não-execução silenciosa:** o teste deve usar `t.Skip` com motivo explícito quando `pwsh` não está disponível no host, nunca PASS silencioso. Um PASS silencioso com `pwsh` ausente é o pior caso — afirma que o guard funciona quando não foi executado.

---

## Seção 5 — Residual Declarado

Este design aceita explicitamente não cobrir os seguintes vetores:

### R1 — Script do instalador não é verificado antes da execução (`irm|iex`)
O checksum SHA-256 protege o binário, não o próprio `install.ps1`. Quem controla a URL da release controla a execução do script. Este risco é idêntico ao `curl | sh` do `install.sh`, já aceito pelo projeto. A mitigação parcial (baixe, inspecione, execute) está documentada na ADR mas não é obrigatória.

### R2 — `tar.exe` path traversal não medido em CI
O bsdtar nativo do Windows descarta `..` e paths absolutos por design do libarchive, mas este comportamento não foi verificado empiricamente no runner `windows-latest` do GitHub Actions. O gate de ML-1A deve adicionar este cenário; até então, a proteção é assumida mas não medida.

### R3 — `PROCESSOR_ARCHITEW6432` em PS 5.1 de 64-bit não medido
O comportamento desta variável em processos PowerShell 5.1 de 64-bit rodando emulados em ARM64 não está documentado com medição direta. A medição em VM ARM64 (ML-4A, AC9) deve confirmar o comportamento real antes do merge.

### R4 — Outro binário `trackfw.exe` mais cedo no PATH
O instalador avisa sobre outras cópias de `trackfw.exe` no PATH mas não as remove. Um binário antigo em `~/bin` do Git Bash (que tipicamente vem antes no PATH) continua respondendo em lugar do novo. Este é o residual explícito da ADR.

### R5 — cmd.exe com a linha PS/POSIX
O comportamento da linha de hook `emulate -R sh 2>${null-/dev/null}; ... exit $LASTEXITCODE` no cmd.exe foi medido como hipótese (ML-0D arm 8 pendente): `; exit $LASTEXITCODE` pode ser interpretado como argumentos pelo cmd.exe em vez de separador, resultando em fail-open. Este defeito é declarado como limitação vigente na REQ e fora do escopo desta entrega.

### R6 — Downgrade attack via `checksums.txt` legítimo de versão antiga
Se o adversário servir simultaneamente o `checksums.txt` e o tarball de uma versão anterior (ambos com hashes corretos entre si), o checksum PASSA mas a versão instalada é a mais antiga. Mitigação parcial: verificar `trackfw.exe --version` após instalação. Mitigação completa requereria assinatura do instalador (fora do escopo desta REQ).

### R7 — Paridade de comportamento de `check-install-ps1.ps1` vs `check-install-checksum.sh`
A paridade de cenários entre os dois gates não é verificada automaticamente. Uma regressão introduzida apenas no `install.ps1` (sem equivalente no `install.sh`) pode não ser detectada até a auditoria do ML-4B.

### R8 — `iex + exit` termina o processo gate no modo `irm|iex`
Medido: `pwsh -NoProfile -NonInteractive -Command "Invoke-Expression 'exit 3'; Write-Host 'AFTER'"` → exitcode=3, "AFTER" não impresso. O arm `irm|iex` do gate deve executar o install.ps1 em processo filho para não terminar o gate. Não verificado em PS 5.1.

### R9 — Instruções corretas apontam para `kgsaran/*`, não `trackfw/*`
As instruções que o ML-3A/3B vai escrever apontam para `brew install kgsaran/trackfw/trackfw` e `go install github.com/kgsaran/trackfw/...`. Os repos `trackfw/homebrew-tap` e `trackfw/trackfw` (404 confirmado) não precisam existir para que essas instruções funcionem. Usuários com as instruções antigas (`trackfw/tap`, `github.com/trackfw/trackfw`) recebem 404 — não hijack — porque a org `trackfw` não tem esses repos. Módulo `github.com/trackfw/trackfw` não está cacheado no proxy Go (medido: `curl -s https://proxy.golang.org/github.com/trackfw/trackfw/@v/list` → "not found: module github.com/trackfw/trackfw").

Caminho residual remanescente: um atacante que comprometesse a org `trackfw` (2FA obrigatório, pertence ao KG) poderia criar `trackfw/homebrew-tap` ou `trackfw/trackfw` e afetar usuários com as instruções antigas nos docs não corrigidos. Por isso ML-3A/3B ainda é necessário: remove as instruções erradas antes que esse caminho tenha impacto.

---

## Impacto no roadmap

Mudanças que o Wave 0 impõe ao roadmap vigente (`ROADMAP-2026-10-28-documentacao-e-canais-refletem-o-produto-11-x-com-suporte-real-a-windows-e-instalador-powershell.md`) e à ADR:

| Artefato | Linha | Mudança exigida |
|---|---|---|
| Roadmap ML-1A | linha 57 (fixture decision) e linha 96 (gate actions) | Decisão tomada: função shadow `Invoke-WebRequest`; sem `BASE_URL`/`DRYRUN` no `install.ps1`; verificação estática no gate; arm `iex` em processo filho |
| `install.ps1` (a ser criado) | — | Não conter `TRACKFW_BASE_URL`, `TRACKFW_MIRROR`, `TRACKFW_INSTALL_DRYRUN`, `Microsoft.PowerShell.Utility\Invoke-WebRequest`, `irm`, `curl.exe`, `wget.exe`, `WebClient`, `HttpClient`, `BitsTransfer`; chamar `Invoke-WebRequest` não-qualificado; regex com `-cmatch '\Av?[0-9]+\.[0-9]+\.[0-9]+(-rc[0-9]{1,3}|-beta[0-9]{1,3})?\z'` |
| `check-install-ps1.ps1` (a ser criado) | — | Verificação estática primeiro; shadow com `[CmdletBinding()]` sem re-declarar common params; log de URLs por arm; arm `iex` em processo filho com shadow redefinido; verificar sob `powershell` (5.1) e `pwsh` (7) no CI |
| Roadmap ML-1B | — | Se `pypi/README.md` for removido do git, `check-wheel-metadata-readme.sh` precisa que `build_wheel.py` leia o root `README.md` (ou o gate o copie antes do build) |
| `npm/README.md` | — | Pode ser removido do git + `.gitignore`; adicionado como tarefa em ML-3A ou ML-1B |
| `pypi/README.md` | — | Pode ser removido do git + `.gitignore`; mesma ressalva de ML-1B acima |
| Seção 2.3 regex | — | Usar `[0-9]`; usar `\z`; usar `-cmatch`; somente `v` minúsculo; sufixo 1-3 dígitos — para ML-1A |

**Não verificado aqui (requer CI Windows):**
- Comportamento de `PROCESSOR_ARCHITEW6432` em PS 5.1 de 64-bit emulado em ARM64 (R3)
- Shadowing de `Invoke-WebRequest` em PS 5.1 (`powershell`) — esperado funcionar (propriedade da linguagem desde PS 2.0), não medido
- `iex + exit` em PS 5.1 (R8)
- `tar.exe` path traversal no runner `windows-latest` (R2)
