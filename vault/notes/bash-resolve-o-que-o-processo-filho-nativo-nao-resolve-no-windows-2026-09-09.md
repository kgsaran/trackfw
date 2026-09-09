# O bash resolve o que o processo filho nativo não resolve (Windows)

> 2026-09-09 · descoberto no ML-R2a/R2c da `REQ-2026-09-03-as-217-falhas-reais-de-windows-...`

## O sintoma

442 das 512 linhas de FAIL do censo de Windows saíam de **uma** causa: `git` não resolvível dentro dos
gates que constroem PATH curado (`check-release-tag-parity.sh`, `check-ship-force-parity.sh`,
`check-push-force-parity.sh`).

## As duas causas, e a segunda é a que engana

### 1. `/usr/bin/git` não existe em nenhum Git for Windows

`BASE_PATH="$RUNTIME_BIN:/usr/bin:/bin"` presume layout de Linux. Medido em ARM64 (VM) **e** em
`windows-latest` x64 (run 34406101512):

```
ls /usr/bin/git /bin/git   → No such file or directory (os dois)
command -v git             → /mingw64/bin/git   (x64)   /clangarm64/bin/git (ARM64)
git --version              → git version 2.55.0.windows.5
```

🔴 **E `/bin` é alias de `/usr/bin`:**

```
cygpath -w /bin     → C:\Program Files\Git\usr\bin
cygpath -w /usr/bin → C:\Program Files\Git\usr\bin
```

Os dois caminhos são **o mesmo diretório**. A redundância que parece rede de segurança ("se não
estiver num, está no outro") é uma entrada só. Em Linux/macOS são distintos — daí o hábito.

### 2. 🔴 `ln -s "$REAL_GIT" "$BIN/git"` cria um link que o bash acha e o filho nativo não

```bash
ln -s "$REAL_GIT" "$GIT_ONLY_BIN/git"     # nome sem .exe
PATH="$GIT_ONLY_BIN" command -v git       # ACHA   (bash/MSYS)
```

Mas `exec.Command("git")` do Go, `spawnSync` do Node e `subprocess.run` do Python **não acham**: no
Windows a resolução é **CreateProcess + PATHEXT**, que exige a extensão. Um arquivo chamado só `git`
nunca é resolvido.

**Por que isso é pior que a causa 1:** falha na direção de *passar*. Um `NO_FORGE_PATH` que existe
para provar ausência genuína de forge CLI, e onde `git` também não resolve, reprova pelo motivo errado
— ou alguma variante passa pelo motivo errado.

## A regra que fica

🔴 **`command -v X` do bash NÃO prova que o produto acha `X` no Windows.** Guarda de não-vacuidade
sobre PATH curado tem de usar **processo filho nativo**:

```bash
PATH="$P" python3 -c 'import subprocess,sys; sys.exit(subprocess.run(["git","--version"],capture_output=True).returncode)'
```

O mesmo vale para qualquer asserção "o binário está no PATH" em teste que roda em Windows.

**Corolário sobre quem lança o quê:** os `ln -s` de `node`/`python3` nos mesmos scripts **não** são
sítios, porque quem os lança é o **bash** do script. Quem procura `git` é o **processo filho nativo**.
O discriminante não é o link — é quem faz a busca.

## Armadilha na correção: não dá para isolar `git.exe` num diretório

Copiar ou hardlinkar `git.exe` sozinho para um diretório novo morre com **`STATUS_DLL_NOT_FOUND`
(0xC0000135)**: as DLLs moram ao lado do executável e a busca de DLL do Windows começa no diretório
dele. A correção tem de acrescentar **o diretório do git**, não um arquivo colocado.

Medido: `/clangarm64/bin` (ARM64) e `/mingw64/bin` (x64) **não** contêm `gh`/`glab`/`az` nem
`sh`/`bash` — então prepender o diretório não quebra o discriminante de ausência de forge CLI.
(`/usr/bin`, esse sim, já traz `sh.exe` e `bash.exe`.)

## Não chame a variável de `GIT_DIR`

`GIT_DIR` é **variável de ambiente reservada do git**: exportada, faz todo `git` seguinte tratar
aquele diretório como o repositório. Neste repositório ela é inclusive tratada como **vetor de
ataque** (`check-gates-falsify.sh`, `falsify/credential-guard-git-env-bypass`). Use `GIT_BIN_DIR`.

## Como reproduzir

Sonda permanente: **Pergunta 12** de `.github/workflows/windows-probe.yml` (`workflow_dispatch`).
