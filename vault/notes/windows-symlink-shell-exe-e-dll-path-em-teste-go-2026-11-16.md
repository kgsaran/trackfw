---
title: "Symlink de shell no Windows: extensão .exe obrigatória e DLL dir no PATH"
date: 2026-11-16
tags: [windows, symlink, dash, msys, go-test, exec]
---

# Symlink de shell MSYS no Windows: dois requisitos não-óbvios

REQ-2026-11-15 (ML-2C). Medição indireta: job 133335668133 (CI PR #688, windows-latest).

## 1. Symlink sem `.exe` não é executável pelo CreateProcess

`os.Symlink(dashPath, "/tmp/xxx/sh")` cria o link, mas
`exec.CommandContext(ctx, "/tmp/xxx/sh", ...)` falha com erro não-ExitError porque o
Windows `CreateProcess` não tenta extensões implícitas (`.exe`, `.bat`, etc.) quando o
chamador fornece um **caminho absoluto completo** — só faz isso para nomes sem slash.

**Sintoma no teste:** `cmd.Run()` retorna um erro que não é `*exec.ExitError`. O padrão
antigo `if exitErr, ok := runErr.(*exec.ExitError)` deixava `code = 0`, e o teste que
esperava exit 2 reprovava com "exit 0 — esperado 2" (duração 0.00s, o que é o sinal).

**Correção:**

```go
shLinkName := "sh"
if runtime.GOOS == "windows" {
    shLinkName = "sh.exe"
}
shLink := filepath.Join(linkDir, shLinkName)
```

`d18ShellKind(shLink)` chama `filepath.EvalSymlinks` e resolve `sh.exe → dash.exe`
(base com prefixo "dash") → retorna `"dash"` corretamente.

## 2. DLL do MSYS não é encontrada se PATH não inclui o diretório do executável real

`buildShellPath(shLink, fakeDir)` incluía apenas `filepath.Dir(shLink)` = TempDir.
O dash do Git for Windows precisa de `msys-2.0.dll`, que fica em `C:\Program Files\Git\usr\bin\`
(o diretório do executável **real**, não do symlink). Sem ele, o processo morre com
`STATUS_DLL_NOT_FOUND` (0xC0000135) — que é um ExitError com código fora de 0–255.

**Prova:** `TestGuardLineShellMatrix_D18ContraArm/*/dash-u` passa no mesmo job
(usa `buildShellPath(shellPath, dir)` onde `shellPath` é o caminho real do dash). O
`ShAsDashSim` falhou porque usava `shLink` em vez de `dashPath` para o buildShellPath.

**Correção:**

```go
// Usar dashPath, não shLink: inclui o diretório das DLLs do dash (Git/usr/bin no Windows).
pathVal := buildShellPath(dashPath, fakeDir)
```

## 3. Padrão defensivo: t.Fatalf para erros que não são ExitError

```go
runErr := cmd.Run()
code := 0
if runErr != nil {
    if exitErr, ok := runErr.(*exec.ExitError); ok {
        code = exitErr.ExitCode()
    } else {
        t.Fatalf("cmd.Run() falhou com erro que não é *exec.ExitError: %v", runErr)
    }
}
// Guarda adicional para NTSTATUS (loader errors):
if code < 0 || code > 255 {
    t.Fatalf("exit %d fora de 0–255 — loader error (shell nunca rodou)", code)
}
```

Sem o `t.Fatalf`, um shell que falha ao iniciar vira `code = 0` silencioso. O teste que
esperava exit ≠ 0 passa (falso verde); o que esperava 2 falha com mensagem enganosa.

## Evidência CI

- `D18ContraArm/*/dash-u` → PASS (usa caminho real, DLLs encontradas)
- `ShAsDashSim/sh-as-dash-u/*` → FAIL com "exit 0 — esperado 2" (0.00s, shell não iniciou)
- `ShAsDashSim/sh-as-dash-e/*` → PASS falso (exit 0 ≠ 2 por acidente)

Pós-correção (macOS, onde dash existe):
- `sh-as-dash-u`: rc=2, stderr="LASTEXITCODE: parameter not set" ✓
- `sh-as-dash-e`: rc=127, sem stderr ✓
