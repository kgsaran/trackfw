# ML-4A — Medição: hook husky sem `sh` no PATH interativo (Windows ARM64)

> Autor: artemis-tf | Data: 2026-11-17
> REQ: REQ-2026-11-15 | ML-4A (parte husky)
> Versão do git medido: 2.55.0.windows.3 (Git for Windows ARM64, clangarm64)
> Versão do husky: 9.1.7 (fonte: `/Users/kgsaran/.npm/_npx/0d92fdd791cc1044/node_modules/husky/husky`)
> VM: `Lab@192.168.64.6` (Windows ARM64 — sem shell interativo diferente de cmd.exe)

---

## Contexto

O achado F1 do parecer do hefesto (`docs/qualidade/2026-11-16-ml-3d-windows-sh-missing-review.md`)
levanta a hipótese de que o wrapper do Git for Windows seta o PATH incluindo `usr\bin` antes de
chamar o interpretador shebang de hooks, o que tornaria `sh` disponível mesmo sem estar no PATH
interativo do usuário. Se verdadeiro, listar `.husky/pre-commit` e `.husky/commit-msg` em
`shConvenienceArtifacts` seria um falso positivo.

Este documento registra a medição que valida ou refuta essa hipótese.

---

## Pré-condições verificadas

### PATH interativo (cmd.exe via SSH)

```
C:\WINDOWS\system32;C:\WINDOWS;C:\WINDOWS\System32\Wbem;
C:\WINDOWS\System32\WindowsPowerShell\v1.0\;C:\WINDOWS\System32\OpenSSH\;
C:\Program Files\Git\cmd;C:\Program Files\Go\bin;C:\Program Files\nodejs\;
C:\Program Files\GitHub CLI\;C:\Users\Lab\AppData\Local\Programs\Python\...
```

Ausências confirmadas:
- `where sh` → "não foi possível localizar" (sh ausente do PATH interativo)
- `where pwsh` → "não foi possível localizar" (pwsh ausente também)
- `C:\Program Files\Git\usr\bin` **não está no PATH**
- `C:\Program Files\Git\bin` **não está no PATH**

`sh.exe` existe em `C:\Program Files\Git\usr\bin\sh.exe` (medido via `dir`), mas não está acessível
ao shell interativo.

### Estrutura Git for Windows (ARM64)

| Diretório | Conteúdo relevante |
|---|---|
| `C:\Program Files\Git\cmd\git.exe` | Wrapper de usuário (45 KB) — no PATH |
| `C:\Program Files\Git\bin\git.exe` | Launcher (45 KB) — fora do PATH |
| `C:\Program Files\Git\clangarm64\bin\git.exe` | Binário nativo ARM64 (3,9 MB) |
| `C:\Program Files\Git\usr\bin\sh.exe` | sh real (2,4 MB) — **fora do PATH interativo** |

### Repositório de teste

```
C:\Users\Lab\ml4a\repo\
├── .git\
│   config   (user.name=testuser, user.email=test@test.com,
│              core.hooksPath=.husky/_)
├── .husky\
│   ├── _\
│   │   ├── pre-commit   ← wrapper husky v9.1.7
│   │   └── commit-msg   ← wrapper husky v9.1.7
│   ├── pre-commit       ← hook do usuário
│   └── commit-msg       ← hook do usuário
└── .hooks-direct\
    └── pre-commit       ← hook direto #!/bin/sh (Braço C)
```

Conteúdo do wrapper (`.husky/_/pre-commit`, idêntico a `.husky/_/commit-msg`):
```sh
#!/usr/bin/env sh
[ "$HUSKY" = "2" ] && set -x
n=$(basename "$0")
s=$(dirname "$(dirname "$0")")/$n
[ ! -f "$s" ] && exit 0
...
export PATH="node_modules/.bin:$PATH"
sh -e "$s" "$@"
c=$?
[ $c != 0 ] && echo "husky - $n script failed (code $c)"
[ $c = 127 ] && echo "husky - command not found in PATH=$PATH"
exit $c
```

Fonte: `node_modules/husky/husky` v9.1.7 (npm: `husky@9.1.7`).

---

## Ações executadas na VM

1. SSH para `Lab@192.168.64.6`; verificação do PATH interativo (`echo %PATH%`) e ausência de sh (`where sh`).
2. `mkdir C:\Users\Lab\ml4a\ml4a\repo` — criação da área de trabalho isolada.
3. `git init` + `git config user.name/email` locais + `git config core.hooksPath .husky/_`.
4. Criação via `scp` de `.husky/_/pre-commit`, `.husky/_/commit-msg` (wrapper husky v9.1.7), `.husky/pre-commit` (hook marcador: `echo ran > marker.txt`), `.husky/commit-msg` (hook marcador: `echo ran-commit-msg > marker-commit-msg.txt`).
5. **Braço A:** `echo init > README.md && git add README.md && git commit -m init`; verificação de `marker.txt`.
6. **Braço B:** scp de `.husky/pre-commit` com `exit 1`; `git commit -m bracoB`; verificação de rc e output.
7. **Braço C:** criação de `.hooks-direct/pre-commit` (`#!/bin/sh; echo ran-binsh > marker-bracoC.txt`); `git config core.hooksPath .hooks-direct`; `git commit -m bracoC`; verificação de `marker-bracoC.txt`.
8. Restauração de `core.hooksPath .husky/_` e do hook marcador original.
9. **Braço D (nativo ARM64):** `echo bracoD > bracoD.txt && git add bracoD.txt && "C:\Program Files\Git\clangarm64\bin\git.exe" commit -m bracoD`; verificação de `marker.txt`.
10. **Braço D (PowerShell):** `powershell.exe -NoProfile -Command "git commit -m bracoD-ps"`; verificação de `marker.txt`.
11. **Braço E:** `echo bracoE > bracoE.txt && git add bracoE.txt && git commit -m bracoE`; verificação de `marker-commit-msg.txt`.
12. **Diagnóstico de PATH:** scp de hook que grava `PATH=$PATH` e `which sh` em `marker-path.txt`; commit + leitura do marker.

### Ações adicionais — corretivo ML-4A (lefthook)

13. **Contra-braço lefthook (fora do hook):** execução de `probe.exe` diretamente do cmd.exe → `LookPath(sh)=NOT_FOUND`; `where sh` → NOT_FOUND; `powershell Get-Command sh` → NOT_FOUND.
14. `mkdir C:\Users\Lab\ml4a\lhprobe` via PowerShell.
15. Escrita de `main.go` (via scp) em `C:\Users\Lab\ml4a\lhprobe\`: Go nativo que faz `exec.LookPath("sh")` e `exec.Command("sh", "-c", "echo sh_exec_ok")` — simula exatamente o que o lefthook binary faz com `runner: sh`.
16. `go mod init lhprobe && "C:\Program Files\Go\bin\go.exe" build -o probe.exe .` — compilação do probe nativo Go ARM64.
17. `mkdir C:\Users\Lab\ml4a\repo-lh` via PowerShell; `git init` + `git config user.name/email` locais.
18. Escrita (via scp) de `.git/hooks/commit-msg`: sh script com 4 braços de registro em `hook-results.txt`: (a) PATH POSIX + `which sh`; (b) `probe.exe` (Go nativo); (c) `cmd.exe /c "where sh"`; (d) `powershell.exe Get-Command sh`.
19. `echo test > test.txt && git add test.txt && git commit -m init` — dispara o hook; leitura de `hook-results.txt`.

---

## Resultados

### Tabela de braços

| Braço | Comando | rc | Marcador criado? | Conteúdo | stderr relevante |
|---|---|---|---|---|---|
| A — husky pre-commit, `#!/usr/bin/env sh` | `git commit -m init` (cmd.exe) | 0 | Sim (`marker.txt`) | `ran` | — |
| B — contra-braço: `exit 1` | `git commit -m bracoB` (cmd.exe) | **≠0** (commit não criado) | Não | — | `husky - pre-commit script failed (code 1)` |
| C — hook direto `#!/bin/sh` (sem husky) | `git commit -m bracoC` (`hooksPath=.hooks-direct`) | 0 | Sim (`marker-bracoC.txt`) | `ran-binsh` | — |
| D — nativo `clangarm64\bin\git.exe` | `clangarm64\bin\git.exe commit -m bracoD` | 0 | Sim (`marker.txt`) | `ran` | — |
| D — PowerShell `powershell.exe -NoProfile` | `git commit -m bracoD-ps` | 0 (commit criado) | Sim (`marker.txt`) | `ran` | nota¹ |
| E — husky commit-msg | `git commit -m bracoE` (cmd.exe) | 0 | Sim (`marker-commit-msg.txt`) | `ran-commit-msg` | — |

¹ O `$?` do PowerShell retornou false, mas o commit `72b6766` foi criado e o marker existia — comportamento de `$?` com `git` em PowerShell é inconsistente. O commit e o hook rodaram.

### Tabela de braços — corretivo lefthook

| Braço | Contexto | `exec.LookPath("sh")` | `exec.Command("sh",…)` |
|---|---|---|---|
| Contra-braço: `probe.exe` direto do cmd.exe | Fora do hook (PATH interativo) | NOT_FOUND | FAILED |
| Contra-braço: `where sh` em cmd.exe | Fora do hook | NOT_FOUND | — |
| Contra-braço: `powershell Get-Command sh` | Fora do hook | NOT_FOUND | — |
| **Go probe dentro de `.git/hooks/commit-msg`** | **Dentro do hook (PATH aumentado)** | **`C:\Program Files\Git\usr\bin\sh.exe`** | **OK (`sh_exec_ok`)** |
| `powershell Get-Command sh` dentro do hook | Dentro do hook | `C:\Program Files\Git\usr\bin\sh.exe` | — |

PATH visto pelo probe Go dentro do hook (Windows format, herdado do ambiente do hook):
```
C:/Program Files/Git/clangarm64/libexec/git-core;C:\Program Files\Git\clangarm64\bin;
C:\Program Files\Git\usr\bin;C:\Users\Lab\bin;C:\WINDOWS\system32;...
```

`C:\Program Files\Git\usr\bin` está presente — adicionado pelo git antes de executar o hook e herdado pelo probe nativo Go.

### PATH visto pelo hook (diagnóstico)

Conteúdo de `marker-path.txt` gravado pelo hook em execução:

```
PATH=node_modules/.bin:/clangarm64/libexec/git-core:/clangarm64/bin:/usr/bin:/c/Users/Lab/bin
  :/c/WINDOWS/system32:/c/WINDOWS:/c/WINDOWS/System32/Wbem
  :/c/WINDOWS/System32/WindowsPowerShell/v1.0:/c/WINDOWS/System32/OpenSSH
  :/cmd:/c/Program Files/Go/bin:/c/Program Files/nodejs:/c/Program Files/GitHub CLI
  :/c/Users/Lab/AppData/Local/Programs/Python/Python312-arm64/Scripts
  :/c/Users/Lab/AppData/Local/Programs/Python/Python312-arm64
  :/c/Users/Lab/AppData/Local/Programs/Python/Launcher
  :%USERPROFILE%/AppData/Local/Microsoft/WindowsApps:%USERPROFILE%/go/bin
  :/c/Users/Lab/AppData/Roaming/npm
/usr/bin/sh
```

Destaques:
- `/usr/bin` **está no PATH do hook** (= `C:\Program Files\Git\usr\bin`) — ausente do PATH interativo
- `/clangarm64/libexec/git-core` e `/clangarm64/bin` — adicionados pelo git
- `node_modules/.bin` — adicionado pelo wrapper husky
- `which sh` → `/usr/bin/sh` (= `C:\Program Files\Git\usr\bin\sh.exe`)

---

## Mecanismo confirmado

Git for Windows (`cmd\git.exe`) não usa o PATH interativo do usuário para executar hooks. O wrapper
augmenta o PATH antes de spawnar o hook, adicionando:
1. `/clangarm64/libexec/git-core` — comandos git internos
2. `/clangarm64/bin` — binários do git ARM64
3. `/usr/bin` — onde `sh.exe`, `bash.exe`, `dash.exe` etc. residem

Isso se aplica tanto ao wrapper `cmd\git.exe` (Braços A, B, E) quanto ao binário nativo
`clangarm64\bin\git.exe` (Braço D) e à invocação via PowerShell (Braço D-PS).

O mecanismo vale para shebangs `#!/usr/bin/env sh` (Git for Windows converte `/usr/bin/env` para o
env real) e `#!/bin/sh` (mapeado para `usr\bin\sh.exe`).

---

## Conclusão

**`.husky/pre-commit` e `.husky/commit-msg` executam sem `sh` no PATH interativo: SIM.**

Condição: Git for Windows instalado (qualquer canal: `cmd\git.exe` wrapper, nativo
`clangarm64\bin\git.exe`, via PowerShell) — o PATH do hook inclui `/usr/bin` (onde `sh.exe` vive)
independentemente do PATH configurado pelo usuário.

**Corretivo ML-4A — lefthook `runner: sh` resolve sh sem sh no PATH interativo: SIM.**

Condição: Git for Windows instalado — o lefthook binary (Go, nativo Windows) é invocado a partir
do `.git/hooks/commit-msg` (sh script instalado pelo lefthook, executado pelo git com PATH
aumentado) e herda o PATH aumentado que inclui `C:\Program Files\Git\usr\bin\sh.exe`. Quando
lefthook executa `runner: sh` via `exec.LookPath("sh")`, encontra sh através do PATH herdado.

A afirmação anterior "lefthook chama `sh` como processo de usuário fora do subshell aumentado" estava
errada: o PATH aumentado é herdado por todos os processos filho do hook, incluindo o lefthook binary
(nativo Go).

---

## Recomendação para `shConvenienceArtifacts`

**Remover `.husky/pre-commit`, `.husky/commit-msg` e `.lefthook/commit-msg/trackfw-req-check.sh`**
da lista em `internal/validator/validator_windows_sh_missing_rule.go:28–35`.

Justificativa: os três são falsos positivos. Qualquer artefato invocado como git hook (diretamente
ou via lefthook) herda o PATH aumentado pelo git, que inclui `/usr/bin` onde vive `sh.exe`. Avisar
que esses hooks "não executam sem sh" é incorreto e contraria a medição direta.

**Lefthook hook template (fonte):** o lefthook (evilmartians/lefthook, Go, qualquer versão >= 1.0)
instala `.git/hooks/commit-msg` como um sh script com shebang `#!/usr/bin/env sh` que chama
`lefthook run commit-msg "$@"`. Git executa esse script com PATH aumentado → lefthook binary herda
PATH aumentado → `runner: sh` (linha `internal/generators/scaffold.go:1834`) resolve sh.
Lefthook não foi instalado na VM; o mecanismo foi verificado via probe Go compilado localmente que
replica exatamente `exec.LookPath("sh")` + `exec.Command("sh", ...)` do lefthook.

**Itens que FICAM na lista** (não são invocados como git hook — usuário os chama diretamente):

| Artefato | Disparado por git hook? | Decisão |
|---|---|---|
| `scripts/trackfw-validate.sh` | Não | MANTER na lista |
| `scripts/trackfw-attention-signal.sh` | Não | MANTER na lista |
| `scripts/trackfw-attention-cleanup.sh` | Não | MANTER na lista |
| `.husky/pre-commit` | Sim (git hook) | REMOVER da lista |
| `.husky/commit-msg` | Sim (git hook) | REMOVER da lista |
| `.lefthook/commit-msg/trackfw-req-check.sh` | Sim (via lefthook, disparado por git hook) | REMOVER da lista |

**Contraponto e limite da medição:** esta medição cobre Git for Windows 2.55.0 ARM64. Há casos
extremos onde o comportamento pode diferir:

| Cenário | Coberto? | Risco residual |
|---|---|---|
| Git for Windows x64 (versão padrão) | Não medido | Baixo — mesmo mecanismo de PATH augmentation documentado pela equipe Git for Windows |
| Git sem PATH augmentation (portable git customizado, git compilado à mão) | Não | Muito raro em uso real |
| Ambientes CI sem Git for Windows (ex: GitHub Actions `windows-latest`) | Não | Actions provisiona Git for Windows com `usr/bin` no PATH — sem risco |
| `core.hooksPath` para `.git/hooks` padrão (sem husky) | Sim (Braço C) | — |
