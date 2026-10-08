---
title: "Red Team: Wave 3 — Credential Guard, caminhos Windows (ML-3C)"
date: 2026-10-08
author: hades-tf
roadmap: ROADMAP-2026-10-06-*.md
ml: ML-3C
req: REQ-2026-10-06
issue: "#544"
branch: fix/credential-guard-caminho-git-bash-windows
commit: f97bcad9
status: Entregue
---

# Red Team — ML-3C: Formas de caminho Windows na 2ª camada do credential guard

> REQ-2026-10-06 · Wave 3 · 2026-10-08 · hades-tf

---

## Veredito

**LIBERA COM RESSALVA.**

Todos os alvos de falsificação do Wave 3 foram exercitados e confirmados na VM (Windows 11 ARM64). Um achado de regressão (F1, LOW) foi encontrado em ADS de stream nomeado — confirmado como aceito pelo Wave 3 §2.7, com evidência numérica agora documentada. O discriminador `::$DATA` (stream padrão = arquivo base) mostra que o branch detecta MELHOR que o main nesse sub-caso, o que suporta a classificação LOW. Dois residuais (O1, R1) não bloqueantes. A Regra Dura de Causa Raiz exige que ambos entrem como ML nesta REQ se forem corrigidos.

---

## Metodologia

**Binário branch (`tf.exe`):** `GOOS=windows GOARCH=arm64 go build ./cmd/trackfw`, commit `f97bcad9`.

**Binário main (`tf-main.exe`):** `git archive origin/main | tar -x -C <scratch>/main-src && GOOS=windows GOARCH=arm64 go build`, mesma versão, sem `credNormalizeWindowsPath` e com `:` na classe de exclusão de `credRedirectRe`.

**Binários de teste:** `GOOS=windows GOARCH=arm64 go test -c -o guard-br.test.exe ./internal/guard/` (branch) e `guard-main.test.exe` (main).

**VM:** Windows 11 ARM64 (`Lab@192.168.64.6`), pasta `C:\Users\Lab\tf-ml3c\`. `TMP=C:\Users\Lab\tf-ml3c\tmp` durante os testes.

**trackfw.yaml block mode:** transferido via scp (LF puro). `mode: block` lido corretamente.

**Payloads:** escritos localmente e transferidos via scp — sem ambiguidade de CRLF do `echo` do cmd.exe.

**Exit code:** capturado via `%errorlevel%` em linha separada (batch com `setlocal enabledelayedexpansion`).

---

## Seção 1 — Suíte completa de testes guard (sem filtro)

Executado com `guard-br.test.exe -test.v -test.timeout 60s` e `guard-main.test.exe -test.v -test.timeout 60s` (sem `-test.run`), cobrindo todos os testes do pacote `internal/guard`.

```
BR_FULL_EXIT=1    ← 1 falha: TestRunGitBranch_NoOpOutsideProject
MAIN_FULL_EXIT=1  ← mesma 1 falha: TestRunGitBranch_NoOpOutsideProject
```

**Falha:** `gitbranch_test.go:379: expected exit 0 outside project, got 2`

Falha **idêntica em branch e main**. Trata-se de o executável localizar `trackfw.yaml` ao subir pela árvore a partir de `C:\Users\Lab\tf-ml3c\proj` (o diretório de trabalho dos testes). Pré-existente, não introduzida por este commit. Não relacionada ao credential guard.

**Testes de credential guard específicos de Windows (10/10):** todos PASS no branch. Main não continha esses testes (adicionados neste commit).

---

## Seção 2 — Comparação main vs. branch

| Caso | main rc | branch rc | Interpretação |
|------|---------|-----------|---------------|
| `cat /c/Users/.../s.env` (sub-caso A, sem cwd) | 0 | **2** | BUG-1 fix: `/c/` → `C:/` |
| `echo hi > C:/Users/.../s.env` (redirect BUG-2) | 0 | **2** | BUG-2 fix: `:` removido, C:/ completo |
| `echo hi > /c/Users/.../s.env` (redirect+BUG-1) | 0 | **2** | BUG-1+2 combinados |
| `echo hi > file.txt:stream` (ADS nomeado; `file.txt` tem secret) | **2** | 0 | VER F1 |
| `echo hi > C:/...s.env::$DATA` (`::$DATA` absoluto) | 0 | **2** | Branch melhora: main perde por BUG-2, branch detecta |
| `echo hi > s.env::$DATA` (`::$DATA` relativo) | **2** | **2** | Ambos detectam |
| non-JSON raw `echo hi > C:/Users/.../s.env` | 0 | **2** | BUG-2 alcança caminho não-JSON |
| `echo KEY > /dev/null 2>&1` | 0 | 0 | F2 isenção preservada |
| pwsh + `/c/Users/.../s.env` | 0 | **2** | pwsh não na deny-list → traduz |
| Warn mode | 0 | 0 | rc=0 + stderr "warning" |

---

## Seção 3 — Vetores FP/FN da tradução `/x/...`

| Vetor | rc | Esperado | Status |
|-------|-----|---------|--------|
| `/c` (bare drive) | 0 | 0 | OK |
| `/cc/x` (dois chars) | 0 | 0 | OK — regex exige 1 letra |
| `/c:` (colon após letra) | 0 | 0 | OK |
| `//c/x` (double slash) | 0 | 0 | OK |
| `/c/../..` (path traversal) | 0 | 0 | OK |
| `/C/Users/.../s.env` (maiúscula) | 2 | 2 | OK |
| sem `tool_name` (Windsurf) | 2 | 2 | OK |
| `tool_name="command_line"` | 2 | 2 | OK |
| `/cygdrive/c/Users/.../s.env` | 2 | 2 | OK |
| cwd=`/c/...` + arg relativo | 2 | 2 | OK |

---

## Seção 4 — Deny-list PowerShell

| Variante | rc | Status |
|----------|-----|--------|
| `"PowerShell"` (exato) | 0 | OK |
| `"powershell"` (lowercase) | 0 | OK — `strings.EqualFold` |
| `"POWERSHELL"` (all caps) | 0 | OK |
| `"pwsh"` | 2 | OK — não está na deny-list → traduz |

---

## Seção 5 — Regressão POSIX

`go test ./internal/...` (macOS): 20 pacotes `ok`. Sem regressão.

`TestCredResolveArg_GitBashPath_POSIXUnchanged`: `/c/Users/x/s.env` com `goos="darwin"` → inalterado. PASS.

`TestCredNormalizeWindowsPath` row `goos="linux"`: `/c/...` → `/c/...` (sem tradução). PASS.

---

## Achados

### F1 — Regressão ADS nomeado: miss quando stream nomeado não existe e arquivo base tem credential (LOW)

**Medição:**
```
Caso:   echo hi > file.txt:stream  (file.txt tem secret, stream não existe)
MAIN:   rc=2  — regex antigo corta em ':', extrai "file.txt", credScanFile detecta
BRANCH: rc=0  — regex novo extrai "file.txt:stream", os.Stat falha, não detecta
```

**Discriminador `::$DATA` (stream padrão = arquivo base):**
```
echo hi > s.env::$DATA  (relativo)
MAIN_ADS_DEFAULT_REL_RC=2   BRANCH: rc=2  — ambos detectam

echo hi > C:/Users/.../s.env::$DATA  (absoluto)
MAIN_ADS_DEFAULT_RC=0       BRANCH: rc=2  — branch melhora (main ainda perde via BUG-2)
```

O discriminador confirma que o problema em F1 é específico de streams nomeados cujo `os.Stat` falha. Para `::$DATA` (stream padrão), o branch detecta corretamente.

**Mecanismo:**
- `echo hi > file.txt:stream` escreve no NTFS ADS de nome `stream` — NÃO no arquivo base `file.txt`
- O secret está em `file.txt` (pré-existente); o comando não o escreve
- Main: truncação acidental pelo antigo regex → escaneava o arquivo errado (o base, não o stream)
- Branch: tenta escanear o alvo correto (`file.txt:stream`), que não existe → `os.Stat` falha → sem detecção

**Wave 3 (ML-3A §2.7) declarou:** "NTFS ADS (file.txt:stream) side-effect: extração completa; os.Stat fails gracefully for unknown stream names — not a functional regression." Evidência numérica agora documentada.

**Classificação:** LOW. ADS redirect com stream nomeado é sintaxe incomum para AI agents. A detecção antiga era um artefato de truncação, não por design. O branch é mais preciso e melhor em `::$DATA`. Não bloqueia merge.

**Ação futura (ML nesta REQ se priorizado):** adicionar fallback em `credResolveArg`: quando o path tem sufixo `:stream-name` (não `:` letra-de-drive nem `::$DATA`) e `os.Stat` falha, tentar também scan do segmento antes do sufixo ADS.

---

### O1 — `NUL` não é isentado como destino de redirecionamento no Windows (INFO, pré-existente)

**Medição:** `echo KEY > NUL` → main rc=2, branch rc=2 (idêntico).

`credAllTargetsAreDevNull()` compara apenas com `/dev/null`. `NUL` (equivalente Windows) não está na lista. Pré-existente, não introduzido por este commit.

**Ação futura (ML nesta REQ se priorizado):** adicionar `NUL` e `nul` à lista de destinos inertes.

---

### R1 — `pwsh` em contexto nativo Windows (PowerShell 7) — residual não medido diretamente

**Situação:** `pwsh` ausente no VM. A afirmação "pwsh emite caminhos POSIX" não foi medida.

**O que foi medido:** `tool_name="pwsh"` + `/c/Users/.../s.env` → branch rc=2. Correto para agente em Git Bash com tool_name="pwsh".

**Risco residual:** se CLI emite `tool_name="pwsh"` com caminhos PowerShell-nativos, a deny-list não o cobre (apenas `"PowerShell"` via EqualFold). Fix: adicionar `"pwsh"` à deny-list — ML nesta REQ se priorizado.

---

## Tabela de reconciliação (testes novos × conclusões do ML-3B)

| Teste | Conclusão do ML-3B que afirma |
|-------|-------------------------------|
| `TestCredNormalizeWindowsPath` — `/c/...` → `C:/...` | Tradução ativa no Windows com toolName≠PowerShell |
| `TestCredNormalizeWindowsPath` — goos=linux → inalterado | Gate GOOS previne FN em POSIX |
| `TestCredNormalizeWindowsPath` — PowerShell → inalterado | Deny-list fecha residual PowerShell |
| `TestRunCredential_GitBashArgSubcaseA_Windows` — rc=2 | Sub-caso A: normalização antes do return |
| `TestRunCredential_GitBashArgSubcaseB_Windows` — rc=2 | Sub-caso B: normalização antes de IsAbs |
| `TestRunCredential_GitBashUppercaseArg_Windows` — rc=2 | Case-insensitive |
| `TestRunCredential_CygdriveArg_Windows` — rc=2 | /cygdrive/c/ coberto |
| `TestRunCredential_GitBashCwd_Windows` — rc=2 | credExtractToolInfoCwd normaliza cwd |
| `TestRunCredential_RedirectWindowsNative_Windows` — rc=2 | BUG-2: `:` removido extrai path completo |
| `TestRunCredential_RedirectGitBash_Windows` — rc=2 | BUG-1+2 combinados |
| `TestRunCredential_PowerShellGitBashPath_Windows` — rc=0 | Deny-list PS: miss esperado |
| `TestRunCredential_DevNullExempt_Windows` — rc=0 | F2 isenção preservada |

---

## Notas de operação

**Chave de exemplo em arquivo de scratch:** `compare.bat` (arquivo temporário em scratchpad da sessão, fora do repositório) continha a chave de exemplo em linha de `echo` de setup, contra a regra "segredo por concatenação". Nenhum artefato do projeto contém o literal. Arquivo de scratch deletado junto com a pasta VM.

**TestRunGitBranch_NoOpOutsideProject:** falha pré-existente em ambos os binários (branch e main), não relacionada ao credential guard.

**VM:** `C:\Users\Lab\tf-ml3c\` deletada e confirmada ausente (`if exist ... echo ABSENT`).
