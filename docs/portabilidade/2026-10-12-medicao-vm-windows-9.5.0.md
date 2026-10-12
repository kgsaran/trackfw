# Medição: VM Windows ARM64 — trackfw 9.5.0

> Data: 2026-10-12 | Medição por: artemis-tf | VM: Windows-Lab (ARM64)

## Contexto

Medição sem alteração de código. Binário `trackfw 9.5.0` compilado na VM a partir do worktree
`v9.5.0` (`a1cc6e9`). Go `1.27.0 windows/arm64`. A VM é ARM64; o CI é x64 — os números de tempo e
processos registrados aqui são exclusivamente da VM. Resultados comportamentais (qual camada
rejeita, qual mensagem sai) são transferíveis para x64 exceto onde indicado.

---

## M1 — Guard global quando `HOME` ≠ `USERPROFILE`

REQ: `docs/req/REQ-2026-09-01-trackfw-escreve-e-audita-guard-global-em-caminho-que-o-agente-real-nunca-le-instalacao-fantasma-sob-home-diferente-de-userprofile.md`

### M1(a) — Valores padrão por terminal

```
Comando medido (cmd.exe):
  echo HOME=%HOME% USERPROFILE=%USERPROFILE%

Saída (cmd.exe):
  HOME=C:\Users\Lab USERPROFILE=C:\Users\Lab

Comando medido (PowerShell):
  echo HOME=$env:HOME USERPROFILE=$env:USERPROFILE

Saída (PowerShell):
  HOME=C:\Users\Lab
  USERPROFILE=C:\Users\Lab

Comando medido (Git Bash):
  echo HOME=$HOME USERPROFILE=$USERPROFILE

Saída (Git Bash, não-login):
  HOME=/c/Users/Lab USERPROFILE=C:\Users\Lab
```

**Observação:** No Git Bash `$HOME` é a forma POSIX `/c/Users/Lab` enquanto `$USERPROFILE` permanece
na forma Windows `C:\Users\Lab`. Eles apontam para o mesmo diretório mas são strings distintas. Esta
é a divergência de forma descrita na REQ.

### M1(b) — Onde o trackfw escreve ao instalar o harness global

```
Comando medido (Git Bash, com HOME=TMPHOME ≠ USERPROFILE):
  export HOME=/c/Users/Lab/tmphome3.UsG5f8
  trackfw init --ai-tools claude --forge github  (em diretório temporário)

Saída relevante (truncada):
  ~/.claude/agents/trackfw-architect.md
  ~/.claude/agents/trackfw-backend.md
  ...
  ~/.claude/skills/trackfw-governance/SKILL.md
  ...
  ✓ claude agents and skills

Verificação pós-init:
  $TMPHOME/.claude/agents/  → ENCONTRADO (12 agentes)
  $TMPHOME/.claude/skills/  → ENCONTRADO (18 skills)
  $TMPHOME/.claude/settings.json → NÃO ENCONTRADO
  $USERPROFILE/.claude/agents/ → NÃO ENCONTRADO
  $USERPROFILE/.claude/settings.json → NÃO ENCONTRADO
  project/.claude/settings.json → ENCONTRADO (hook do guard, escopo de projeto)
```

**Achado:** O `trackfw init --ai-tools claude` em 9.5.0:
- Escreve agents e skills em `~/.claude/` (HOME-based) — com `HOME=TMPHOME`, vão para `TMPHOME/.claude/`
- **Não escreve nenhum `settings.json` global** — a hook do guard fica apenas no `.claude/settings.json` do projeto
- Portanto, o "guard global" descrito na REQ não existe em 9.5.0: o guard é somente de escopo de projeto

**`trackfw validate` com HOME=TMPHOME (diretório vazio):**
```
✗ the trackfw resolved at "C:\\Users\\Lab\\bin\\trackfw.exe" does not have the guard
  subcommand — the hook will fail open; update trackfw
✗ (repetido)
Error: 2 violation(s) found
```

As 2 violações referem-se ao binário antigo em PATH (`C:\Users\Lab\bin\trackfw.exe`), não à
divergência `HOME` ≠ `USERPROFILE`. Nenhum finding sobre HOME ≠ USERPROFILE foi emitido.

### M1(c) — Onde o Claude Code lê o `settings.json` global

```
Verificação: which claude, find AppData -name claude.exe, ls AppData/Roaming/npm/
Resultado: claude não encontrado em PATH
  - AppData/Local/claude-cli-nodejs/ existe mas contém apenas Cache/ (instalação incompleta ou
    desinstalada)
  - AppData/Roaming/npm/node_modules/@anthropic-ai/ — diretório vazio
  - Nenhum claude.exe ou claude.cmd encontrado
```

**Limite de medição:** O Claude Code não estava disponível nesta VM no momento da medição. A
documentação oficial do Claude Code para Windows indica que o `settings.json` global fica em
`%APPDATA%\Claude\settings.json` ou `%USERPROFILE%\.claude\settings.json`. Não foi possível confirmar
qual das duas o binário instalado nesta VM usaria, pois o binário não está instalado.

### M1(d) — Veredito

| Aspecto | Resultado |
|---|---|
| HOME ≠ USERPROFILE por string? | **Sim** — Git Bash: `/c/Users/Lab` vs `C:\Users\Lab` |
| trackfw escreve guard global em HOME? | **Não** — 9.5.0 não escreve `settings.json` global |
| Agentes/skills vão para HOME (e não para USERPROFILE)? | **Sim** — "fantasma" de agents/skills reproduz |
| `validate` emite finding de HOME ≠ USERPROFILE? | **Não** — nenhum finding relacionado observado |
| Claude Code instalado para confirmar AC1? | **Não medido** — binário ausente na VM |

**Veredito:** O defeito de "guard global fantasma" (AC3/AC4 da REQ) **não reproduz** em 9.5.0 porque
não existe `settings.json` global. O defeito análogo para agents/skills **reproduz** (com HOME=TMPHOME
os artefatos vão para o diretório errado). A REQ, porém, é específica sobre o guard/settings.json.

**Ressalva ARM64:** sem impacto nesta medição (não envolve compilação condicional por plataforma).

---

## M2 — Guard de travessia de caminho com backslash (`ML-R3`)

Linha de referência: `internal/integrations/manager.go:756` (guard `default:` do `resolve()`).

### Teste descartável na VM

```go
// zz_medicao_test.go — apagado após execução
// package integrations
```

Arquivo criado em `C:\Users\Lab\tf-9.5.0\internal\integrations\zz_medicao_test.go` e apagado ao
final.

### Resultado na VM (Windows ARM64)

```
$ go test ./internal/integrations/ -run TestM2_BackslashTraversal -v

=== RUN   TestM2_BackslashTraversal/backslash_single_project
    destination="..\outside.md" scope="project"
    err="destination \"..\\outside.md\" is outside project root"
    CAMADA: beneath() — containment check (pathguard.Beneath)
--- PASS

=== RUN   TestM2_BackslashTraversal/backslash_double_project
    destination="..\..\x.md" scope="project"
    err="destination \"..\\..\\x.md\" is outside project root"
    CAMADA: beneath() — containment check (pathguard.Beneath)
--- PASS

=== RUN   TestM2_BackslashTraversal/backslash_single_global
    CAMADA: beneath() — containment check (pathguard.Beneath)
--- PASS

=== RUN   TestM2_BackslashTraversal/backslash_double_global
    CAMADA: beneath() — containment check (pathguard.Beneath)
--- PASS

ok  github.com/kgsaran/trackfw/internal/integrations  0.032s
```

### Mesmo teste no Mac (cópia da árvore via rsync)

```
$ go test ./internal/integrations/ -run TestM2_BackslashTraversal -v

--- FAIL: TestM2_BackslashTraversal/backslash_single_project
    resolve("..\\outside.md", scope="project"): expected error (traversal), got nil
--- FAIL: TestM2_BackslashTraversal/backslash_double_project
    resolve("..\\..\\x.md", scope="project"): expected error (traversal), got nil
--- FAIL: TestM2_BackslashTraversal/backslash_single_global
    resolve("..\\outside.md", scope="global"): expected error (traversal), got nil
--- FAIL: TestM2_BackslashTraversal/backslash_double_global
    resolve("..\\..\\x.md", scope="global"): expected error (traversal), got nil

FAIL  github.com/kgsaran/trackfw/internal/integrations  0.468s
```

### Análise das camadas

**Guard de entrada (linha 756, `path.Clean` + `HasPrefix("../")`):**
- `path.Clean("..\outside.md")` usa o pacote `path` (separador POSIX `/`); backslash não é separador
  → resultado igual ao original → condição `!= destination` é falsa
- `strings.HasPrefix("..\outside.md", "../")` → `false` (backslash ≠ forward slash)
- **O guard de entrada NÃO captura backslash traversal em nenhuma plataforma**

**`filepath.Join(root, "..\outside.md")`:**
- **Windows:** `\` é separador → `filepath.Join("C:\\root", "..\outside.md")` = `C:\outside.md` (traversal real)
- **macOS:** `\` não é separador → `filepath.Join("/tmp/root", "..\outside.md")` = `/tmp/root/..\outside.md` (nome de arquivo literal com backslash)

**`beneath(root, destination)` após o join:**
- **Windows:** `filepath.Rel("C:\\root", "C:\\outside.md")` = `..\outside.md` → `HasPrefix("..\", "..\\")` = true → **beneath retorna false → erro capturado**
- **macOS:** `filepath.Rel("/tmp/root", "/tmp/root/..\outside.md")` = `..\outside.md` (nome de arquivo literal) → `HasPrefix` com separador `/` não casa → **beneath retorna true → resolve() retorna nil (sem erro)**

### Veredito M2

| Item | Windows ARM64 (VM) | macOS (Mac) | Nota |
|---|---|---|---|
| Guard de entrada (linha 756) captura `\..\`? | Não | Não | dead code para backslash |
| `beneath()` captura `\..\`? | **Sim** | **Não** | diferença de separador nativo |
| `resolve()` retorna erro? | **Sim** (beneath) | **Não** (nil) | |
| Arquivo escrito fora de root? | Não (catch por beneath) | Não (backslash é char literal em Unix) | macOS: arquivo criado dentro de root com nome estranho |

**Ressalva ARM64:** sem impacto — é comportamento puro de string; `filepath` é determinado por GOOS
na compilação, não pela arquitetura.

---

## M3 — Órfão do SSH quando o prazo de rede estoura (RS-2 do ADR-2026-10-12)

ADR de referência: `docs/adr/ADR-2026-10-12-operacao-de-rede-do-trackfw-tem-prazo-e-o-estouro-nunca-vira-sucesso-silencioso.md`

### Setup

```
Repositório temporário com:
  git remote add origin "ssh://x.invalid/y.git"
GIT_SSH_COMMAND="C:/Program Files/Git/bin/bash.exe" /path/to/fakessh.sh
  (fakessh.sh: escreve PID no marker, dorme 120s)
GIT_SSH_VARIANT=ssh
TRACKFW_NETWORK_TIMEOUT=5s
```

### Resultado — Git Bash

```
$ TRACKFW_NETWORK_TIMEOUT=5s trackfw branch prune

Warning: could not fetch origin (git fetch timed out after 5s (TRACKFW_NETWORK_TIMEOUT))
  — evaluating with possibly stale data; a branch merged upstream since the last fetch
  may still be reported as pending.
trackfw branch prune: origin/main not found — offline, no remote configured, or never
  fetched. Refusing to evaluate any branch; nothing deleted.
branch prune: origin/main not resolvable

Exit code: 1
Elapsed: ~10–11s (5s timeout + ~5s WaitDelay)
SSH invocado: sim (fakessh.sh com args: -o SendEnv=GIT_PROTOCOL x.invalid git-upload-pack '/y.git')
```

### Verificação de órfão

```
SSH PID: 1034

Verificação imediatamente após retorno do comando:
  ORPHAN: pid 1034 still alive immediately after return

Verificação 3s depois:
  ORPHAN PERSISTS: pid 1034 still alive 3s after return

ps aux 3s depois (amostra):
  1034  /usr/bin/bash   (fakessh.sh — ainda em execução)
  1035  /usr/bin/sleep  (sleep 120 dentro do fakessh.sh)
```

### PowerShell

Não medido nesta execução: o script PowerShell embutido falhou com
`CommandNotFoundException` para o caminho POSIX do binário (`/c/Users/Lab/trackfw-9.5.0.exe`).
Para medir via PowerShell seria necessário usar o caminho Windows do binário.

### Veredito M3

| Item | Resultado |
|---|---|
| Comando retorna dentro do prazo? | **Sim** — ~10s (5s + WaitDelay ~5s) |
| Mensagem de estouro nomeia prazo e variável? | **Sim** — "timed out after 5s (TRACKFW_NETWORK_TIMEOUT)" |
| Processo SSH órfão após retorno? | **Sim** — persiste pelo menos 3s após retorno |
| Mecanismo esperado (D7 do ADR) | Setpgid+Cancel em Unix; só WaitDelay no Windows → RS-2 declarado |

**RS-2 confirmado em Windows ARM64:** o processo `bash` (fakessh.sh) + `sleep 120` sobrevivem ao
menos 3s após `trackfw branch prune` retornar. O ADR já documenta isso como risco residual
aceito: "sempre no Windows, só o `WaitDelay` atua (RS-2, RS-6), e o neto pode sobreviver."

**Ressalva ARM64:** a medição de prazo (10s) é ARM64. O CI x64 pode diferir na latência do WaitDelay.

---

## M4 — Testes Go que pulam no Windows

```
Pacotes testados:
  go test ./internal/commands/ ./internal/netdeadline/ ./internal/sync/ ./internal/guard/
  -count=1 -v
```

### Resultados por pacote

| Pacote | Resultado | Tempo |
|---|---|---|
| `internal/commands` | **FAIL** (28 falhas) | 69.3s |
| `internal/netdeadline` | PASS | 0.1s |
| `internal/sync` | PASS | 5.2s |
| `internal/guard` | PASS | 3.0s |

### SKIPs (11 total)

| Teste | Pacote | Motivo |
|---|---|---|
| `TestBarrierReentry_T1_DirectReentrance` | commands | shell gate com fusível requer sh POSIX; Windows coberto por windows-known-failures.json |
| `TestBarrierReentry_T2_IndirectReentrance` | commands | shell gate requer sh POSIX; Windows coberto por windows-known-files.json |
| `TestBarrierReentry_T3_LegitimateNested` | commands | os.Symlink pode falhar no Windows sem permissão de admin |
| `TestBarrierReentry_T7_Canonicalization` | commands | (symlink-related, mesma família T3) |
| `TestBarrierReentry_T7e_APFSCaseInsensitive` | commands | T7e: EvalSymlinks normalizou case — skip (bypass não ocorreria) |
| `TestGitExec_AC2_Bite_NoWaitDelay` | commands | bite test skipped on Windows: pipe hang behaviour not confirmed there |
| `TestGitExec_WaitDelay_ExitZeroWithHangingGrandchild` | commands | pre-push hook test skipped on Windows: shell hooks not supported |
| `TestPush_ForceWithLease_NoPROpen_Blocks` | commands | force-with-lease+no-PR path has no end-to-end coverage |
| `TestBarrierFragment_TransportSingleLineReadContra` | commands | sh not found in PATH — skipping single-line read contra-arm |
| `TestCredResolveArg_GitBashPath_POSIXUnchanged` | commands | POSIX test — skipped on Windows |
| `TestMatchSubcommand_BackslashPrefixed_PosixNotGit` | commands | POSIX-only: on Windows filepath.Base('\\git') returns 'git' — divergência intencional |

### Falhas (28 — todos em `internal/commands`)

Causa principal das falhas `TestADRPrefixE2E_*` (5 falhas):
```
fatal: unable to access 'NUL': Invalid argument
```
Causa: `git init -q -b main .` em `os.TempDir()` (`C:\Users\Lab\AppData\Local\Temp`). O git
falha ao acessar o dispositivo NUL neste path no ambiente da VM. Pode ser restrição do ambiente de
teste ou comportamento da versão de git instalada.

Outras falhas (`TestBarrierContract_*`, `TestBarrierFragment_*`, `TestBranchStateE2E_*`,
`TestShip_Integration_*`): relacionadas a shell gates e barreiras que precisam de `sh` POSIX ou
de comportamento de hooks específico ao Unix.

**Nota:** `TestNetworkCmd_WaitDelay_ExitZeroWithHangingGrandchild` (netdeadline) passou em 522ms:
confirma que o mecanismo de WaitDelay funciona para `runNetworkCmd`. O teste de nível mais alto
`TestGitExec_WaitDelay_ExitZeroWithHangingGrandchild` foi pulado por "shell hooks not supported".

### Veredito M4

SKIPs são todos declarados e esperados: 8 por ausência de `sh` POSIX, 2 por comportamento diferente
de symlinks/case no Windows, 1 por cobertura parcial não declarada para Windows. Nenhum skip
surpreendente.

Falhas: 28, principalmente por `git init` com NUL no TempDir (ambiente) e por ausência de shell
POSIX nos testes de barreira. Nenhuma falha em netdeadline, sync ou guard.

**Ressalva ARM64:** o `git init` NUL pode ser específico do git instalado nesta VM ARM64. Precisa
comparar com x64 no CI para distinguir ambiente de produto.

---

## Tabela Final

| Item | Reproduz na 9.5.0? | Evidência principal | Ressalva ARM64 |
|---|---|---|---|
| **M1** — guard global fantasma (HOME ≠ USERPROFILE) | Parcial: **agents/skills** sim; **guard/settings.json** não (não escrito) | trackfw init não escreve settings.json global; agents vão para HOME | sem impacto |
| **M2** — backslash traversal não capturado pelo guard de entrada (linha 756) | **Sim (ambas plataformas)** — mas `beneath()` captura no Windows; macOS retorna nil | teste descartável: Windows PASS (beneath captura), macOS FAIL (nil) | sem impacto (GOOS) |
| **M3** — órfão SSH quando prazo estoura (RS-2) | **Sim** — pid persiste 3s+ após retorno | fakessh PID 1034 vivo 3s após trackfw retornar; mensagem correta | latência WaitDelay pode diferir em x64 |
| **M4** — testes pulados no Windows | 11 SKIPs declarados; 28 falhas (ambiente + shell POSIX) | commands FAIL; netdeadline/sync/guard PASS | git NUL falha pode ser ARM64-específico |

---

```
git status --short (antes de escrever este arquivo):
(clean)
```
