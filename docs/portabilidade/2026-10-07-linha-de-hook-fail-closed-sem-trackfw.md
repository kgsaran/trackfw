# Linha de hook fail-closed quando `trackfw` está ausente do PATH (ML-6A)

> Data: 2026-10-07
> REQ: REQ-2026-09-05 (issue #535)
> Branch: `fix/hooks-de-guard-executam-no-windows-trackfw-ausente`
> Medido por: ares-tf

## Contexto

Issue #535 reporta que `trackfw guard git-branch; exit $LASTEXITCODE` sem o binário no PATH deixa o
guard **fail-open**: bash sai 127 (comando não encontrado), PowerShell 5.1 sai 0 (`$LASTEXITCODE`
não é atualizado quando o comando não existe → `exit $null` = `exit 0`). Os CLIs de agente que
bloqueiam somente no exit 2 deixam tudo passar.

Este documento mede qual linha de hook é **fail-closed** (sai 2 quando o binário está ausente) sem
perder o comportamento normal (guard nega → 2; guard libera → 0).

Contexto adicional:
- ADR `docs/adr/ADR-2026-10-04-o-guard-de-hook-e-um-subcomando-go-do-trackfw-...` D2: linha de hook
  é a mesma string em todo shell.
- Doc `docs/seguranca/2026-10-04-wave0-guard-em-go.md` linha 358 afirma: "String fail-closed
  universal impossível: `|| exit 2` não funciona em PS 5.1 (documentado)." — esta medição
  **confirma** essa afirmação para `|| exit 2` especificamente, mas **encontra um polyglot que
  funciona** por mecanismo diferente.

---

## Ambiente

### macOS (host)
- Platform: darwin arm64
- `/bin/sh`: bash em modo POSIX (macOS Ventura+)
- `/bin/bash`: bash 3.2.57 (macOS system)
- Binário Go compilado: `go build -o <scratchpad>/ml6a/trackfw ./cmd/trackfw` → trackfw 9.3.1

### Windows VM
- UTM ARM64, Windows 11, `ssh Lab@192.168.64.6`
- PowerShell: 5.1.26100.9549 (confirmado: `$PSVersionTable.PSVersion.Major = 5`)
- Git Bash: `/usr/bin/bash` via `C:\Program Files\Git\bin\bash.exe`
- Binário Go: cross-compiled `GOOS=windows GOARCH=arm64` → trackfw 9.3.1
  - Copiado para `C:\Users\Lab\ml6a\bin\trackfw.exe` via scp
- Instalações existentes na VM (deliberadamente excluídas dos cenários A):
  - `C:\Users\Lab\bin\trackfw.exe` (8.0.0-rc2, instalação antiga do GitHub release)
  - pip: `C:\Users\Lab\AppData\Local\Programs\Python\Python312-arm64\Scripts\trackfw.exe` (9.2.0)
  - npm: `C:\Users\Lab\AppData\Roaming\npm\trackfw.cmd` / `trackfw.ps1`
- `ExecutionPolicy: Restricted` (padrão da VM)

### Payloads
- **DENY**: `{"tool_input":{"command":"git push origin main"}}` — guard nega, espera exit 2
- **ALLOW**: `{"tool_input":{"command":"ls"}}` — guard libera, espera exit 0

### PATH por cenário
| Cenário | PATH macOS | PATH Windows |
|---|---|---|
| **A** (binário ausente) | `/usr/bin:/bin` (sem homebrew) | `C:\Windows\System32;...;PS_PATH` (sem ml6a\bin, sem Lab\bin, sem pip/npm) |
| **B/C** (binário presente) | `<scratchpad>:/usr/bin:/bin` | `C:\Users\Lab\ml6a\bin;C:\Windows\System32;...;PS_PATH` |

**Nota PATH Git Bash:** o Git Bash nesta VM adiciona `/c/Users/Lab/bin` (trackfw 8.0.0-rc2) ao PATH
— hipótese: via variável de ambiente permanente do Windows (não verificado com `bash -c 'echo
$PATH'` sob `set PATH=%SYS_PATH%`, apenas observado o efeito). O PATH mínimo para os testes foi
forçado via `export PATH=...` dentro dos wrappers bash (v9.bat/run_poly_absent.sh), não apenas via
`set PATH=` do cmd.exe.

---

## Candidatos testados

| ID | Linha exata | Família pretendida |
|---|---|---|
| **C1** | `trackfw guard git-branch; exit $LASTEXITCODE` | PS/POSIX (linha atual) |
| **C2** | `command -v trackfw >/dev/null 2>&1 \|\| exit 2; trackfw guard git-branch` | POSIX puro |
| **C3** | `trackfw guard git-branch \|\| exit 2` | POSIX + cmd |
| **C3b** | `trackfw guard git-branch \|\| exit /b 2` | cmd alternativo |
| **C4** | `try { trackfw guard git-branch } catch { exit 2 }; exit $LASTEXITCODE` | PS 5.1 puro |
| **C5** | `trackfw guard git-branch; if (-not $?) { exit 2 }; exit $LASTEXITCODE` | PS 5.1 puro |
| **Ca** | `trackfw guard git-branch; e=$?; [ $e -ne 0 ] && exit 2; exit 0` | sh/bash puro |
| **Cpoly** | `$LASTEXITCODE=2; trackfw guard git-branch; LASTEXITCODE=$((2*!!$?)); exit $LASTEXITCODE` | **polyglot PS/POSIX** |

---

## Matriz de resultados

Formato: `A / B / C` onde A = ausente, B = deny, C = allow. Meta: `2 / 2 / 0`.

### macOS: sh -c

| Candidato | A | B | C | Resultado |
|---|---|---|---|---|
| C1 | 127 | 2 | 0 | **FAIL** (A=127 ≠ 2) |
| C2 | 2 | 2 | 0 | **PASS** |
| C3 | 2 | 2 | 0 | **PASS** |
| C5 | 2 | 2 | 2 | **FAIL** (parse error → deny-all) |
| Ca | 2 | 2 | 0 | **PASS** |
| **Cpoly** | **2** | **2** | **0** | **PASS** (stderr: `=2: command not found`) |

C5 em `/bin/sh`: syntax error em `{` → exit 2 para TODOS os cenários — deny-all.
(Medição inicial mostrava A=0 instável por interferência de pipe; medição direta confirma 2/2/2.)

### macOS: bash -c

| Candidato | A | B | C | Resultado |
|---|---|---|---|---|
| C1 | 127 | 2 | 0 | **FAIL** (A=127) |
| C2 | 2 | 2 | 0 | **PASS** |
| C3 | 2 | 2 | 0 | **PASS** |
| C5 | 2 | 2 | 2 | **FAIL** (parse error → deny-all) |
| Ca | 2 | 2 | 0 | **PASS** |
| **Cpoly** | **2** | **2** | **0** | **PASS** (stderr: `=2: command not found`) |

### Windows: Git Bash não-login (bash -c / bash script.sh com PATH forçado)

| Candidato | A | B | C | Resultado |
|---|---|---|---|---|
| C1 | 127 | 2 | 0 | **FAIL** (A=127) |
| C2 | 2 | 2 | 0 | **PASS** |
| C3 | 2 | 2 | 0 | **PASS** |
| C4 | 127 | — | — | **FAIL** (`try: command not found` → fail-open) |
| C5 | 2 | 2 | 2 | **FAIL** (parse error → deny-all) |
| Ca | 2 | 2 | 0 | **PASS** |
| **Cpoly** | **2** | **2** | **0** | **PASS** (stderr: `=2: command not found`) |

### Windows: Git Bash login (-lc) — C3 e Cpoly

| Candidato | A | B | C | Resultado |
|---|---|---|---|---|
| C3 | 2 | 2 | 0 | **PASS** |
| Cpoly | — | — | — | não medido (equivalente ao não-login com PATH forçado) |

Login shell não altera o resultado quando o PATH é forçado via `export PATH=...` dentro do comando.

### Windows: PowerShell 5.1 (powershell.exe -NoProfile -Command "..." com cd testdir)

| Candidato | A | B | C | Resultado |
|---|---|---|---|---|
| C1 | 0 | 2 | 0 | **FAIL** (A=0, fail-open — confirma issue #535) |
| C2 | 1 (ParseError) | — | — | **FAIL** (`\|\|` causa ParseError em PS 5.1) |
| C3 | 1 (ParseError) | 1 (ParseError) | 1 (ParseError) | **FAIL** (ParseError — guard nunca executa) |
| C4 | 2 | 2 | 0 | **PASS** (confirmado com bare `trackfw` via PATH) |
| C5 | 2 | 2 | 0 | **PASS** (confirmado com bare `trackfw` via PATH) |
| Ca | 1 (ParseError) | 1 (ParseError) | 1 (ParseError) | **FAIL** (`[` e `&&` causam ParseError em PS 5.1) |
| **Cpoly** | **2** | **2** | **0** | **PASS** (stderr: CommandNotFound para `LASTEXITCODE=$((2*!!$?))`) |

**Nota crítica sobre C3/Ca em PS 5.1:** a ParseError impede que a linha execute **em qualquer
cenário** — o binário nunca chega a rodar. Todos os cenários (A, B e C) saem com exit 1 do
`powershell.exe`. Para CLIs que bloqueiam somente no exit 2, exit 1 = fail-open para todos os
comandos. Isso torna C3 **pior que o atual C1** em PS 5.1 puro.

C4 e C5: confirmados com `trackfw` resolvido via PATH (não caminho explícito).

**Cpoly em PS:** mecanismo é diferente das famílias PS-pura e POSIX-pura:
- `$LASTEXITCODE=2` é uma atribuição PS válida (pré-semente)
- `trackfw guard git-branch` — se ausente: CommandNotFoundException (não atualiza `$LASTEXITCODE`) → pré-semente sobrevive
- `LASTEXITCODE=$((2*!!$?))` (sem `$` na LHS) — **não é atribuição em PS** — é nome de comando → CommandNotFoundException (não atualiza `$LASTEXITCODE`)
- `exit $LASTEXITCODE` — sai com o valor que ficou após trackfw: 2 (ausente/deny) ou 0 (allow)

### Windows: cmd /c

| Candidato | A | B | C | Resultado |
|---|---|---|---|---|
| C1 (sem sufixo) | 1 | — | — | **FAIL** (A=1 ≠ 2) |
| C3 (`\|\| exit 2`) | 2 | 2 | 0 | **PASS** |
| C3b (`\|\| exit /b 2`) | 2 | 2 | 0 | **PASS** (equivalente a C3 para `cmd /c`) |
| C2 (`command -v \|\|`) | 2 | 2 | 2 | **FAIL** (deny-all — `command` não existe em cmd) |
| C1 (com sufixo) | 1 | 2 | 2 | **FAIL** (A=1 fail-open; B/C=2 deny-all via ADR D7) |
| **Cpoly** | **1** | **1** | **1** | **FAIL** (fail-open para todos — `$LASTEXITCODE` é nome de comando em cmd) |

C3 e C3b confirmados sem parênteses (bare `trackfw guard git-branch || exit 2`) via v6.bat.
C2 em cmd: `command -v` não é um comando do cmd — deny-all confirmado.

**C1 com sufixo em cmd (ADR D7):** quando o binário está presente, `cmd /c "trackfw guard
git-branch; exit $LASTEXITCODE"` não separa em dois comandos — `;` não é separador em cmd. O
trackfw recebe `guard` com argumentos `git-branch;`, `exit`, `$LASTEXITCODE`. Cobra tenta despachar o
subcomando `git-branch;` (com ponto-e-vírgula) → desconhecido → **exit 2 para TODOS os cenários**
(deny-all, incluindo allow). Quando ausente (A): trackfw não encontrado → ERRORLEVEL=1.

**Cpoly em cmd:** `$LASTEXITCODE=2` não é sintaxe PS nem atribuição em cmd — cmd tenta executar
`$LASTEXITCODE=2` como nome de comando (sem encontrá-lo) → ERRORLEVEL=1. Como `;` também não é
separador em cmd, a linha inteira é tokenizada como comando único a partir do `$`, e todo o resto
(incluindo o `trackfw`) fica como argumento desse comando inexistente. O trackfw **nunca é
invocado**. A=1, B=1, C=1: fail-open para todos os cenários.

---

## Verificação do afirmado na wave 0

A linha 358 de `docs/seguranca/2026-10-04-wave0-guard-em-go.md` afirma:

> "String fail-closed universal impossível: `|| exit 2` não funciona em PS 5.1 (documentado)."

**PARCIALMENTE CONFIRMADO.** `|| exit 2` (C3) não funciona em PS 5.1 — confirmado por medição
direta com erro literal `O token '||' não é um separador de instruções válido nesta versão.`

Porém, a afirmação de que **nenhuma string fail-closed universal existe** foi **falsificada** por
esta medição: o candidato Cpoly funciona em sh, bash, Git Bash e PS 5.1 com resultados corretos.

Evidência literal do PS 5.1 para C3:
```
O token '||' não é um separador de instruções válido nesta versão.
    + FullyQualifiedErrorId : InvalidEndOfLine
```
Exit code = 1 para TODOS os cenários.

Comportamento do PS 5.1 com `||` e `&&`:
1. `||` e `&&` → ParseError (`InvalidEndOfLine`) — impede execução em qualquer cenário (A, B, C)
2. `&&` → ParseError idêntico
3. `[` → ParseError adicional: "Nome de tipo ausente depois de '['." (PS 5.1 trata `[` como cast de tipo)

---

## Vereditos

### (a) Existe linha única PS/POSIX com A=2, B=2, C=0 em sh, bash, Git Bash e PS 5.1?

**Sim. Candidato Cpoly:**

```
$LASTEXITCODE=2; trackfw guard git-branch; LASTEXITCODE=$((2*!!$?)); exit $LASTEXITCODE
```

| Shell | A | B | C | Resultado |
|---|---|---|---|---|
| macOS sh | 2 | 2 | 0 | **PASS** |
| macOS bash | 2 | 2 | 0 | **PASS** |
| Git Bash não-login | 2 | 2 | 0 | **PASS** |
| PS 5.1 | 2 | 2 | 0 | **PASS** |

**Custo:** stderr ruído em toda execução:
- bash/sh: `=2: command not found` (da expansão `$LASTEXITCODE=2` com LASTEXITCODE unset)
- PS 5.1: `LASTEXITCODE=$((2*!!$?)) : O termo '...' não é reconhecido...` (CommandNotFoundException)

**Mecanismos paralelos:**

| Aspecto | bash/sh | PS 5.1 |
|---|---|---|
| `$LASTEXITCODE=2` | `=2: command not found` (exit 127, irrelevante) | atribuição válida: `$LASTEXITCODE=2` |
| Ausente (A) | exit 127 → `$?=127` → `2*!!127=2` → exit 2 | CommandNotFound, `$LASTEXITCODE` não atualizado → permanece 2 (pré-semente) → exit 2 |
| Deny (B) | exit 2 → `$?=2` → `2*!!2=2` → exit 2 | exit 2 → `$LASTEXITCODE=2` → pré-semente sobrescrita pelo valor correto → exit 2 |
| Allow (C) | exit 0 → `$?=0` → `2*!!0=0` → exit 0 | exit 0 → `$LASTEXITCODE=0` → exit 0 |
| `LASTEXITCODE=$((2*!!$?))` | atribuição bash aritmética válida | **CommandNotFoundException** (sem `$` na LHS) → `$LASTEXITCODE` não atualizado |

**Ressalva de exit 1 em PS:** se trackfw sair com exit 1 (erro, não deny), `$LASTEXITCODE=1` e
o Cpoly sai 1 em PS (não 2). Para CLIs que bloqueiam somente no exit 2, isso seria fail-open para
erros de trackfw. C5 (`if (-not $?)`) é mais estrito: qualquer exit ≠ 0 vira exit 2. Avaliar por
política de segurança do projeto.

**Vantagem Cpoly em bash para binário obsoleto:** se um binário antigo (ex: trackfw 8.0.0-rc2,
sem o subcomando `guard`) estiver em PATH, cobra sai 1 (exit "unknown command" pré-D7). Em bash,
`2*!!1=2` → Cpoly sai 2 (fail-closed). O C1 original sairia 1 (fail-open para CLIs com critério
exit-2). Em PS, porém, `$LASTEXITCODE=1` após o trackfw obsoleto; o Cpoly sai 1 (fail-open).

**Precondição PS 5.1 — escopo global obrigatório:** Cpoly funciona em PS **somente em escopo
global** (direto via `-Command "<linha>"`). Em **script blocks** (`& { <linha> }`), `-File` ou
`.ps1`, a atribuição `$LASTEXITCODE=2` cria uma variável **local** no escopo do bloco. O trackfw
é invocado e atualiza o `$LASTEXITCODE` **global** (0 para allow) — mas dentro do bloco, o `exit
$LASTEXITCODE` lê o local (ainda 2) → **deny-all** (C=2 em vez de 0).

Medido com `& { $LASTEXITCODE=2; trackfw guard git-branch; LASTEXITCODE=$((2*!!$?)); exit $LASTEXITCODE }` em PS 5.1:

| Cenário | Exit | Esperado | Correto? |
|---|---|---|---|
| A (ausente) | 2 | 2 | ✓ |
| B (deny) | 2 | 2 | ✓ |
| C (allow) | 2 | 0 | ✗ DENY-ALL |

**Cpoly em cmd:** não funciona — trackfw nunca é invocado (ver seção cmd acima).

### (b) Melhor por CLI, cruzando com o shell que cada CLI usa para executar hooks

**Distinção essencial:** `hookName: "PreToolUse:PowerShell"` indica qual ferramenta o modelo estava
usando quando o hook disparou — **não** qual shell executa o comando do hook.

Medido em ML-5B (sessão `3f932985`, Claude Code 2.1.292, Windows ARM64): o Claude Code executa
comandos de hook via `/usr/bin/bash` (Git Bash), mesmo quando o evento é `PreToolUse:PowerShell`.
Evidência direta: o hook com `exit %ERRORLEVEL%` produziu `/usr/bin/bash: line 1: exit:
%ERRORLEVEL%: numeric argument required`.

**Nota ADR vs ML-5C:** O ADR (linha 30) lista Codex como usando PowerShell. O ML-5C documental
classifica Codex como "não verificável" no Windows (fonte OpenAI cita `Bash` como canônico, sem
distinção por OS). Como o Cpoly funciona em ambas as famílias, a ambiguidade não afeta a
recomendação.

| CLI | Shell de execução do hook | Fonte | Candidato recomendado | A / B / C |
|---|---|---|---|---|
| Claude Code | Git Bash (`/usr/bin/bash`) | medido ML-5B sessão `3f932985` | **Cpoly** | 2 / 2 / 0 |
| Codex CLI | não verificado (ADR: PS; docs: Bash) | ADR linha 30 + ML-5C | **Cpoly** (funciona em ambos) | 2 / 2 / 0 |
| Gemini CLI | não verificado em runtime | ML-5C documental | **Cpoly** ¹ | 2 / 2 / 0 |
| GitHub Copilot | não verificado em runtime | ML-5C documental | **Cpoly** ¹ | 2 / 2 / 0 |
| Cursor | não verificado em runtime | ML-5C documental | **Cpoly** ¹ | 2 / 2 / 0 |
| Windsurf | não verificado em runtime | ML-5C documental | **Cpoly** ¹ | 2 / 2 / 0 |
| Kiro | cmd.exe | ML-5C documental | **C3** `trackfw guard git-branch \|\| exit 2` | 2 / 2 / 0 |
| Amazon Q | não verificado (docs: `execute_bash`) | ML-5C documental | **C3** (default cmd; rever se confirmado bash) | 2 / 2 / 0 |

¹ **Ressalva para família PS:** Cpoly requer que o CLI execute o hook via `-Command "<linha>"` em
escopo global PS. Se o CLI usar `-Command "& { <linha> }"`, `-File` ou `.ps1`, Cpoly é **deny-all**
(C=2 em vez de 0) por scoping de variáveis PS. Nenhum desses CLIs foi medido em runtime — a coluna
"A/B/C" pressupõe escopo global. Se comportamento for confirmado como script block, substituir por
C4 ou C5 (PS-only).

**Nota sobre Claude Code:** o ADR diz que Claude Code pode usar Git Bash **ou** PowerShell
dependendo da máquina. Em máquinas sem Git Bash, Claude Code usaria PowerShell para hooks. O Cpoly
funciona em ambos — sem necessidade de string diferente por máquina. Em PS, requer escopo global
(ver ¹ acima).

### (c) Família cmd (Kiro): melhor candidato

**C3: `trackfw guard git-branch || exit 2`**

| Cenário | Exit | OK? |
|---|---|---|
| A (binário ausente) | 2 | ✓ |
| B (binário presente, deny) | 2 | ✓ |
| C (binário presente, allow) | 0 | ✓ |

cmd.exe suporta `||` como "execute se anterior falhou". `exit /b 2` equivale a `exit 2` para
a invocação `cmd /c "..."`. O Cpoly não funciona em cmd (`;` não é separador).

---

## Resumo da matriz completa

| Candidato | macOS sh | macOS bash | Git Bash | Git Bash login | PS 5.1 | cmd | Universal PS/POSIX? |
|---|---|---|---|---|---|---|---|
| C1 (atual) | FAIL 127 | FAIL 127 | FAIL 127 | — | FAIL 0 | FAIL 1 | Não |
| C2 (`command -v \|\|`) | PASS | PASS | PASS | — | FAIL ParseErr | DENY-ALL | Não |
| C3 (`\|\| exit 2`) | PASS | PASS | PASS | PASS | FAIL ParseErr exit 1 | PASS | Não |
| C3b (`\|\| exit /b 2`) | — | — | — | — | FAIL ParseErr | PASS | Não |
| C4 (try/catch) | — | FAIL 127 | FAIL 127 | — | PASS | — | Não (PS only) |
| C5 (if -not $?) | DENY-ALL | DENY-ALL | DENY-ALL | — | PASS | — | Não (PS only) |
| Ca (`e=$?;[…]&&`) | PASS | PASS | PASS | — | FAIL ParseErr | FAIL 1 | Não (sh/bash only) |
| **Cpoly** | **PASS** | **PASS** | **PASS** | — | **PASS** ² | FAIL 1/1/1 | **Sim (bash+PS global)** |

**Legenda:** DENY-ALL = syntax error → exit 2 para todos os cenários incluindo allow.
FAIL ParseErr = ParseError → exit 1; fail-open para CLIs com critério exit 2.
Cpoly FAIL em cmd: `$LASTEXITCODE=2` é nome de comando em cmd → ERRORLEVEL=1; trackfw nunca invocado.
² PS: PASS somente em escopo global (`-Command` direto). Script blocks (`& { }`) → DENY-ALL (C=2).

---

## Artifacts de medição

- Binário macOS: `go build -o <scratchpad>/ml6a/trackfw ./cmd/trackfw` → trackfw 9.3.1
- Binário Windows: `GOOS=windows GOARCH=arm64 go build -o <scratchpad>/ml6a/trackfw.exe ./cmd/trackfw` → trackfw 9.3.1
- Scripts de teste Windows: `ml6a_v2.bat` (cmd/Git Bash/PS inicial), `ml6a_v6.bat` (PS B/C bare, cmd C3 sem parens, cmd C2 B/C, Git Bash login, cmd exit /b 2), `ml6a_v9.bat` + `run_poly_absent.sh` + `run_poly_present.sh` (polyglot Cpoly), `ml6a_v10.bat` (cmd Cpoly A/B/C, cmd C1-with-suffix A/B/C, PS script block Cpoly A/B/C)
- poly.sh: `$LASTEXITCODE=2; trackfw guard git-branch; LASTEXITCODE=$((2*!!$?)); exit $LASTEXITCODE`
- trackfw.yaml de teste: `C:\Users\Lab\ml6a\testdir\trackfw.yaml` (req_dir/roadmap_dir minimal)
- Pasta limpa no fim: `C:\Users\Lab\ml6a` apagada; bat files de home apagados (incluindo v10.bat)
