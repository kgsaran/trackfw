# credential guard: formas de caminho Windows na Layer 2 — dois bugs independentes

> Encontrado: 2026-10-08 · REQ-2026-10-06 · ML-3A · hades-tf · issue #544
> Status: NÃO corrigido (fix previsto no ML-3B)

## Causa raiz

Dois bugs independentes na Layer 2 do credential guard fazem com que caminhos no formato Git Bash e caminhos nativos Windows com letra de unidade em redirects passem sem detecção.

### BUG-1: caminhos Git Bash (`/c/...`) não resolvem no Windows — dois sub-casos

**Sítio:** `credResolveArg` (credential.go:499–514)

```go
clean = strings.TrimRight(clean, "},")
if clean == "" { return arg }
if baseCwd != "" && !filepath.IsAbs(clean) {  // ← sub-caso B
    return filepath.Join(baseCwd, clean)
}
return clean                                   // ← sub-caso A retorna aqui
```

**Sub-caso A (baseCwd="" — sem `tool_info.cwd` no payload):** `if baseCwd != ""` é falso → path `/c/Users/...` devolvido como-está → `os.Stat("/c/Users/...")` → Windows interpreta `/c/` como `C:\c\` → arquivo não encontrado. `filepath.IsAbs` NÃO é consultado neste sub-caso.

**Sub-caso B (baseCwd="C:\\..." — cwd nativo no payload):** `filepath.IsAbs("/c/...")=false` → `filepath.Join("C:\\...", "/c/.../s.env")` = `C:\c\...\s.env` (caminho errado) → stat falha.

**Afeta:** todos os chamadores de `credResolveArg` — Layer 2b (arg de cat/head/tail/jq/grep), Layer 2a (redirect), `credDeepScan`, `credNonJSONLayerTwoB`.

**Afeta também:** `credExtractToolInfoCwd` — se cwd do payload é `/c/...`, argumentos relativos produzem `filepath.Join("/c/cwd", "relative.env")` = `/c/cwd/relative.env` → stat falha.

### BUG-2: `credRedirectRe` exclui `:` da classe de caracteres

**Sítio:** credential.go:30

```go
credRedirectRe = regexp.MustCompile(`[0-9]?>>?[ \t]*[^ \t\r\n|&;,:]+`)
```

`:` está na classe de exclusão. Para `echo > C:\Users\...` ou `echo > C:/Users/...`, o regex extrai apenas `C` (a letra de unidade). `credScanFile("C")` busca um arquivo chamado `C` no cwd — que normalmente não existe.

**Prova direta (VM):** criado arquivo `C` com chave no cwd. Payload `echo > C:\nonexistent.env` → rc=2 detected=true. Confirma que o guard achou o arquivo errado (`C`), não o destino real.

## Medição resumida (Windows 11 ARM64, v9.3.3, block mode)

### Layer 2b (arg de cat)

| Forma | Detecta? |
|-------|---------|
| `C:\\...` (JSON-esc) | ✅ |
| `C:/...` | ✅ |
| `/c/...` (Git Bash) | 🔴 NÃO |
| `/C/...` (Git Bash upper) | 🔴 NÃO |
| `/cygdrive/c/...` | 🔴 NÃO |
| `s.env` (relativo) | ✅ |

### Layer 2a (redirect)

| Forma | Detecta? | Causa |
|-------|---------|-------|
| `C:\\...` (JSON-esc) | 🔴 NÃO | BUG-2: regex extrai `C` |
| `C:/...` | 🔴 NÃO | BUG-2: idem |
| `/c/...` | 🔴 NÃO | BUG-2 não se aplica; regex extrai caminho completo; mas BUG-1 falha |
| `s.env` (relativo) | ✅ | |

## Prova POSIX: não é afetado

`filepath.IsAbs("/c/Users/...")` = `true` no macOS/Linux. A fix para BUG-1 (tradução `/X/...` → `X:\...`) DEVE ser gateada em `runtime.GOOS == "windows"`.

## Fix proposto para ML-3B

1. Função `credNormalizeWindowsPath(path, goos, toolName string) string`:
   - `/[a-zA-Z]/...` → `X:/<rest>` (forward slash — `filepath.FromSlash` é OS-dependente nos testes; `C:/...` funciona: §2.2 medido)
   - `/cygdrive/[a-zA-Z]/...` → `X:/<rest>` (forward slash, idem)
   - Chamada em `credResolveArg`: **após** `strings.TrimRight` e o check de empty, **antes** da linha `if baseCwd != "" && !filepath.IsAbs` (credential.go linha 510). Isso cobre sub-caso A e sub-caso B.
   - Chamada em `credGlobToken`: antes de `filepath.Glob`.
   - Chamada em `credExtractToolInfoCwd`: normaliza o cwd extraído.

2. Remover `:` de `credRedirectRe`: `[^ \t\r\n|&;,]+` (sem `:`). Efeito lateral para NTFS ADS: `file.txt:ads` é extraído completo → stat falha graciosamente (sem regressão funcional).

**UNC:** detecta quando admin share acessível (C$ habilitado na VM, medido: rc=2 detected=true). Sem defeito estrutural.

**Warn vs block:** modo warn detecta os mesmos casos que block (FN por `/c/...` existe em ambos); apenas rc muda (0 em vez de 2) e a linha de stderr muda ("warning" em vez de "blocked").

**PowerShell path residual (duas direções; parcialmente fechado):** `[System.IO.Path]::GetFullPath('/c/Users/Lab')` = `C:\c\Users\Lab` na VM. Gate deny-list `strings.EqualFold(toolName, "PowerShell")` fecha para Claude Code (`"PowerShell"` confirmado: `docs/portabilidade/2026-10-04-...:646`). **Confirmado aberto para Codex:** Codex emite `tool_name="Bash"` mesmo em PowerShell (ROADMAP-2026-09-22...:440); forma dos caminhos que escreve não medida — se usar `/c/...`, deny-list traduz incorretamente. Windsurf (sem tool_name) cai no deny-list — não medido. `credNormalizeWindowsPath(path, goos, toolName string) string` — só strings/regexp; saída forward-slash; `credExtractCmdAndCwd(data []byte) (shellCmd, cwd, toolName string, isJSON bool)` — assinatura atual lida.

## Residual

Caminhos MSYS sem letra de unidade (`/tmp`, `/home`, `/usr`) não são cobertos pela regra de tradução. Pré-existente.
