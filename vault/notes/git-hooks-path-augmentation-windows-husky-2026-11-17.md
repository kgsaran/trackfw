# Git for Windows augmenta PATH ao rodar hooks — husky executa sem sh no PATH interativo

**Data:** 2026-11-17
**Medido por:** artemis-tf, ML-4A, REQ-2026-11-15
**VM:** Windows ARM64, Git for Windows 2.55.0.windows.3, husky 9.1.7

## Achado

Git for Windows (qualquer executável: `cmd\git.exe`, `clangarm64\bin\git.exe`, via PowerShell)
**augmenta o PATH antes de executar git hooks**, adicionando:

```
/clangarm64/libexec/git-core
/clangarm64/bin
/usr/bin          ← aqui vive sh.exe (C:\Program Files\Git\usr\bin\sh.exe)
```

Isso significa que hooks com `#!/usr/bin/env sh`, `#!/bin/sh`, e scripts que chamam `sh -e` (como
o wrapper husky) **executam corretamente mesmo quando `sh` não está no PATH interativo do usuário**.

## Prova direta

A medição criou um hook que gravou `PATH=$PATH` e `which sh` num marker:

```
/usr/bin/sh
```

PATH interativo não continha `C:\Program Files\Git\usr\bin`. O PATH dentro do hook continha `/usr/bin`.

## Impacto em `shConvenienceArtifacts`

**`.husky/pre-commit` e `.husky/commit-msg` são falsos positivos** na lista
`internal/validator/validator_windows_sh_missing_rule.go`. Devem ser removidos da lista.

**`.lefthook/commit-msg/trackfw-req-check.sh` também é falso positivo** — corretivo ML-4A
(2026-11-17): a afirmação anterior de que "lefthook chama `sh` fora do subshell aumentado" estava
errada. Medição direta com probe Go nativo compilado em `C:\Users\Lab\ml4a\lhprobe\` e executado
dentro de `.git/hooks/commit-msg` mostrou:

```
LookPath(sh)=C:\Program Files\Git\usr\bin\sh.exe
exec(sh)=OK output=sh_exec_ok
PATH=C:\Program Files\Git\usr\bin;...  (Windows format, aumentado)
```

Mecanismo: lefthook instala `.git/hooks/commit-msg` como sh script (`#!/usr/bin/env sh`) que
invoca o lefthook binary. Git aumenta PATH antes de rodar o hook → lefthook binary (Go, nativo)
herda PATH aumentado → `runner: sh` (`exec.LookPath("sh")`) encontra `sh.exe`.

**Itens que FICAM na lista** (não invocados por git hook):
`scripts/trackfw-validate.sh`, `scripts/trackfw-attention-signal.sh`,
`scripts/trackfw-attention-cleanup.sh`.

## Versões medidas

| Componente | Versão |
|---|---|
| Git for Windows | 2.55.0.windows.3 (ARM64) |
| husky | 9.1.7 |

## Limite

Medido apenas em Git for Windows ARM64. Versões x64 usam o mesmo mecanismo de PATH augmentation
(documentado pela equipe Git for Windows) — mas não foi medido diretamente neste ML.

## Links

- Medição completa: `docs/portabilidade/2026-11-17-ml-4a-husky-sem-sh-no-path.md`
- Achado F1 do hefesto (que levantou a hipótese): `docs/qualidade/2026-11-16-ml-3d-windows-sh-missing-review.md`
- Lista que deve ser atualizada: `internal/validator/validator_windows_sh_missing_rule.go:28–35`
