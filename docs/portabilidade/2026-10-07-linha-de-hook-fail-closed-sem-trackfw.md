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

---

## Rodada 2 — silenciar o polyglot (2026-10-07)

> Objetivo: eliminar o ruído de stderr nos cenários B e C do Cpoly (mantendo o REASON do guard em B),
> com A aceitando erro curto.
> Meta: 2/2/0 nos mesmos 4 shells e stderr vazio em B/C.

### Binários e ambiente

- Binário macOS: `go build -o <scratchpad>/ml6a2/trackfw ./cmd/trackfw` → trackfw 9.3.1
- Binário Windows: `GOOS=windows GOARCH=arm64 go build -o <scratchpad>/ml6a2/trackfw.exe ./cmd/trackfw` → trackfw 9.3.1
- VM: `C:\Users\Lab\ml6a2\` (limpo ao final)
- testdir: `trackfw.yaml` minimal + `deny.json` + `allow.json`

### Análise de compatibilidade de redirecionamento (pré-medição)

Antes de testar candidatos, foi levantada a matriz de viabilidade de redirecionamento cross-shell:

| Redirect | macOS sh/bash | Git Bash (Windows) | PS 5.1 (Windows) |
|---|---|---|---|
| `2>/dev/null` | ✓ null device | ✓ MinGW null device | ✗ `DirectoryNotFoundException` (`C:\dev\null` não existe) |
| `2>nul` | Cria arquivo `nul` no diretório (silencia, mas efeito colateral) | ✓ Windows null device | ✗ `NotSupportedException` ("FileStream foi solicitado a abrir um dispositivo que não era um arquivo") |
| `2>$null` (`null` unset) | ✗ "ambiguous redirect" (bash expande `$null` = vazio → `2>""`) | ✗ idem | ✓ redireciona para stream nulo PS |
| `2>$null` (`null=/dev/null`) | ✓ funciona | ✓ funciona | ✓ funciona |

**Conclusão da análise**: não existe redirect sintético único que seja um null device válido em bash/sh E PS 5.1.
A única combinação que funcionaria exigiria pré-setar `null=/dev/null` em bash — mas esse comando (`null=/dev/null`)
é CommandNotFound em PS, gerando novo ruído.

### Candidatos testados

| ID | Linha | Raciocínio |
|---|---|---|
| **R1** | `$LASTEXITCODE=2 2>/dev/null; trackfw guard git-branch; LASTEXITCODE=$((2*!!$?)); exit $LASTEXITCODE` | Redirecionar ruído da semente em bash via `2>/dev/null` |
| **R_EAP** | `$ErrorActionPreference='SilentlyContinue'; $LASTEXITCODE=2; trackfw guard git-branch; LASTEXITCODE=$((2*!!$?)); exit $LASTEXITCODE` | Suprimir CommandNotFound em PS via `$ErrorActionPreference` |
| **C2-colon** | `: $LASTEXITCODE=2; trackfw guard git-branch; LASTEXITCODE=$((2*!!$?)); exit $LASTEXITCODE` | Substituir semente por `:` (no-op bash, silencia `=2:`) |
| **C_NULL** | `$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard git-branch; LASTEXITCODE=$((2*!!$?)); exit $LASTEXITCODE` | `${null-/dev/null}`: em bash = `/dev/null` (param expansion); em PS = variável nula → redirect silencioso |

### Matriz de resultados

Formato: `exit / noise_msgs_stderr`. Meta: `2 / 0` em B e C (exceto REASON em B).

#### R1 — `$LASTEXITCODE=2 2>/dev/null; ...`

| Shell | A | B | C |
|---|---|---|---|
| macOS sh | 2 / `trackfw: not found` | 2 / 0 (só REASON) ✓ | 0 / 0 ✓ |
| macOS bash | 2 / `trackfw: not found` | 2 / 0 (só REASON) ✓ | 0 / 0 ✓ |
| Git Bash não-login | 2 / `trackfw: not found` (inferido¹) | 2 / 0 (só REASON) (inferido¹) | 0 / 0 (inferido¹) |
| **PS 5.1** | **0 (FAIL-OPEN)** / `out-file: DirectoryNotFoundException` | 2 / 2 msgs (`out-file: DirectoryNotFoundException` + `LASTEXITCODE=...: CommandNotFound`) | 0 / 2 msgs (idem) |

¹ Git Bash usa bash com `/dev/null` disponível (MinGW); comportamento idêntico ao macOS bash, conforme padrão estabelecido na Rodada 1. `${null-/dev/null}` = `/dev/null` confirmado diretamente no Git Bash da VM nesta rodada.

**Correção Rodada 1 (PS B/C)**: a medição inicial de B/C PS mostrava `head -c 200` truncado. A medição completa (Rodada 2) confirma 2 mensagens de ruído por cenário: `out-file: DirectoryNotFoundException` + `LASTEXITCODE=$((2*!!$?)): CommandNotFound`.

**Mecanismo PS**: `$LASTEXITCODE=2 2>/dev/null` em PS tenta abrir `C:\dev\null` para escrita antes de executar
a atribuição. Como `C:\dev\` não existe, `out-file` lança `FileOpenFailure / DirectoryNotFoundException`.
A atribuição **não executa** — `$LASTEXITCODE` permanece 0. Sem a semente, o cenário A sai 0 (fail-open).

**Nota sobre EAP e DirectoryNotFoundException**: `$ErrorActionPreference='SilentlyContinue'` **suprime a mensagem** do `FileOpenFailure` (medido: sem output de erro, execução continua). Mas a **atribuição ainda não executa** — o redirect falhado aborta a declaração inteira antes de executar o lado esquerdo. Resultado: mesmo com EAP, A é fail-open e B/C ficam silenciosas mas com semente inoperante. EAP e o redirect `2>/dev/null` são mutuamente exclusivos: não existe combinação que resolva bash e PS ao mesmo tempo (detalhado na seção "Combinações tentadas").

**Veredito R1**: Silencia bash B e C perfeitamente (**meta atingida para bash**), mas QUEBRA PS —
falha na abertura do redirect impede a semente, tornando A fail-open. Não viável como linha única cross-shell.

#### R_EAP — `$ErrorActionPreference='SilentlyContinue'; $LASTEXITCODE=2; ...`

| Shell | A | B | C |
|---|---|---|---|
| macOS sh | 2 / 3 msgs (`=SilentlyContinue:` + `=2:` + `trackfw:`) | 2 / 2 msgs (`=SilentlyContinue:` + `=2:`) + REASON | 0 / 2 msgs (`=SilentlyContinue:` + `=2:`) |
| macOS bash | 2 / 3 msgs | 2 / 2 msgs + REASON | 0 / 2 msgs |
| Git Bash não-login | 2 / 3 msgs (inferido) | 2 / 2 msgs + REASON (inferido) | 0 / 2 msgs (inferido) |
| **PS 5.1** | **2 / 0 ✓** | **2 / 0 (só REASON) ✓** | **0 / 0 ✓** |

**Mecanismo PS**: Confirmado que `$ErrorActionPreference='SilentlyContinue'` suprime erros
CommandNotFoundException (não terminantes) em PS 5.1. Após EAP setado:
- `$LASTEXITCODE=2` → atribuição válida (silenciosa) ✓
- `trackfw guard git-branch` ausente → CommandNotFound suprimido por EAP ✓
- `LASTEXITCODE=$((2*!!$?))` → CommandNotFound suprimido por EAP ✓
- `exit $LASTEXITCODE` → sai com o valor da semente ou do trackfw ✓

**Mecanismo bash**: `$ErrorActionPreference='SilentlyContinue'` expande `$ErrorActionPreference`
(não setado → vazio) → tenta executar comando `='SilentlyContinue'` → `command not found`. NOVO ruído.
Total: 2 mensagens de ruído em B e C (vs. 1 do Cpoly original).

**Veredito R_EAP**: Silencia PS totalmente (meta PS atingida), mas **piora bash** — de 1 mensagem de
ruído por cenário (Cpoly) para 2. Troca de perspectiva, não melhoria global.

#### C2-colon — `: $LASTEXITCODE=2; ...`

| Shell | A | B | C |
|---|---|---|---|
| macOS sh | 2 / `trackfw: not found` (1 msg) | 2 / 0 (só REASON) ✓ | 0 / 0 ✓ |
| macOS bash | 2 / `trackfw: not found` (1 msg) | 2 / 0 (só REASON) ✓ | 0 / 0 ✓ |
| **PS 5.1** | **0 (FAIL-OPEN)** / `: CommandNotFound` | 2 / `: CommandNotFound` + REASON | 0 / `: CommandNotFound` |

**Mecanismo bash**: `:` é builtin no-op do bash. Seus argumentos são expandidos mas descartados silenciosamente.
`: $LASTEXITCODE=2` → `:` recebe argumento `=2` (expansão com LASTEXITCODE não setado) → descartado sem ruído.
`$LASTEXITCODE` NÃO é setado (não é necessário — bash usa `$?` na aritmética).
B e C ficam limpos.

**Mecanismo PS**: `:` não é cmdlet/função PS → CommandNotFound. `$LASTEXITCODE` não é setado.
Sem semente, A = fail-open (0). Idêntico ao problema do R1.

**Veredito C2-colon**: Resolve o ruído da semente em bash para B e C (melhor que Cpoly para bash),
mas QUEBRA PS da mesma forma que R1 — sem semente, A é fail-open.

#### C_NULL — `$LASTEXITCODE=2 2>${null-/dev/null}; ...`

**Mecanismo de `${null-/dev/null}`**:
- **bash/sh**: `${null-/dev/null}` é parameter expansion. `null` não está definido → substitui pelo default `/dev/null`. Resultado: `2>/dev/null`. Confirmado em macOS bash, macOS sh e Git Bash da VM (`echo ${null-/dev/null}` → `/dev/null`).
- **PS 5.1**: `${null-/dev/null}` em PS resolve para variável `null-/dev/null` (braces permitem caracteres especiais em nomes de variável PS). A variável é indefinida → valor nulo/empty. `2>$empty` em PS → redireciona para o stream nulo PS (sem criar arquivo, sem erro). Confirmado: `Write-Error x 2>${null-/dev/null}; Write-Host after` → output `after`, sem erro, sem arquivo criado, exit 0.

| Shell | A | B | C |
|---|---|---|---|
| macOS sh | 2 / `trackfw: not found` (1 msg) | 2 / 0 (só REASON) ✓ | 0 / 0 ✓ |
| macOS bash | 2 / `trackfw: not found` (1 msg) | 2 / 0 (só REASON) ✓ | 0 / 0 ✓ |
| Git Bash não-login | 2 / `trackfw: not found` (1 msg) ²✓ | 2 / 0 (só REASON) ✓ ² | 0 / 0 ✓ ² |
| **PS 5.1** | **2 / 2 msgs** (`trackfw:CommandNotFound` + `LASTEXITCODE=...:CommandNotFound`) | 2 / 1 msg (`LASTEXITCODE=...:CommandNotFound`) | 0 / 1 msg (`LASTEXITCODE=...:CommandNotFound`) |

² Git Bash A medido diretamente na VM: `cnull_test.sh` → exit=2, `=2:` ruído silenciado (TRACKFW_EC=127, sem mensagem de redirect), 1 msg de ruído restante (`trackfw: command not found`). B/C idênticos ao macOS bash (mesmo engine, `/dev/null` confirmado).

**PS mecanismo C_NULL**:
- `$LASTEXITCODE=2 2>${null-/dev/null}` → `${null-/dev/null}` é null/empty em PS → redirect para stream nulo → **atribuição executa** (`$LASTEXITCODE=2`) sem ruído ✓
- Diferente de R1 onde `2>/dev/null` causava DirectoryNotFoundException que abortava a atribuição.
- PS A: `$LASTEXITCODE=2` seed sobrevive; `trackfw` ausente → CommandNotFound (não atualiza `$LASTEXITCODE`) → sai 2 ✓
- PS B/C: idêntico ao Cpoly (semente depois sobrescrita pelo trackfw exitcode; `LASTEXITCODE=$((2*!!$?))` é CommandNotFound mas não altera `$LASTEXITCODE`) ✓

**Veredito C_NULL**: **Estritamente melhor que Cpoly para bash B/C** (0 noise vs 1). PS fica com noise idêntico ao Cpoly (1 msg por cenário B/C). Correctness completa em todos os 4 shells. A semente sobrevive em PS porque `${null-/dev/null}` resolve para null-redirect (não arquivo), diferente de `2>/dev/null` (R1) que tenta abrir `C:\dev\null`.

### Combinações tentadas e por que falham

**R1 + R_EAP** (redirect bash + EAP PS):
`$ErrorActionPreference='SilentlyContinue'; $LASTEXITCODE=2 2>/dev/null; trackfw guard git-branch; LASTEXITCODE=$((2*!!$?)); exit $LASTEXITCODE`

- bash: `$ErrorActionPreference='SilentlyContinue'` ainda gera ruído (não tem redirect). 1 ruído restante em B/C.
- PS: EAP setado, mas `$LASTEXITCODE=2 2>/dev/null` → PS tenta abrir `C:\dev\null`. Com EAP=SilentlyContinue, a mensagem de erro é suprimida — mas **a atribuição ainda não executa** (o redirect falhado aborta o statement inteiro antes de executar a LHS). Medido: `$ErrorActionPreference='SilentlyContinue'; $LASTEXITCODE=2 2>'/dev/null'; Write-Host ('AFTER, LASTEXITCODE=' + $LASTEXITCODE)` → `AFTER, LASTEXITCODE=` (vazio), exit 0. EAP silencia a mensagem; a semente ainda falha → A fail-open.

**C2-colon + R_EAP** (`: $ErrorActionPreference='SilentlyContinue'` como no-op bash + EAP PS):
Teria de ser `: $ErrorActionPreference='SilentlyContinue'; $LASTEXITCODE=2; ...`
- bash: `: $ErrorActionPreference=...` → `:` builtin, silencioso ✓. `$LASTEXITCODE=2` → ruído ainda presente.
- PS: `:` → CommandNotFound antes de EAP ser setado. EAP não chega a ser ativado.

**Raiz da incompatibilidade para R1/C2-colon**: O redirect `2>/dev/null` é necessário para silenciar bash, mas
quebra o mecanismo de semente em PS (statement abortado, mesmo com EAP). O EAP é necessário para
silenciar PS, mas é invisível ao bash (que trata o prefixo como comando inexistente, gerando ruído adicional).
As duas soluções exigem mecanismos de shells opostos, sem sintaxe que satisfaça os dois simultaneamente.

**C_NULL escapa dessa incompatibilidade**: `${null-/dev/null}` usa mecanismos distintos em cada shell
sem conflito — em bash é parameter expansion (`/dev/null`), em PS é variável undefined (null-redirect).
Resultado: semente executa em PS (sem erro) e ruído é silenciado em bash (sem afetar PS).

### Tabela consolidada de trade-offs

| Candidato | bash B/C noise | PS B/C noise | Correctness (A=2 em todos) |
|---|---|---|---|
| **Cpoly (baseline)** | 1 msg (`=2:`) | 1 msg (`LASTEXITCODE=...:`) | ✓ |
| **R1** | **0** ✓ | 2 msgs (`out-file:` + `LASTEXITCODE=...:`) + A fail-open | ✗ PS A=0 |
| **R_EAP** | **2 msgs** ✗ (pior) | **0** ✓ | ✓ |
| **C2-colon** | **0** ✓ | 1 msg + A fail-open | ✗ PS A=0 |
| **C_NULL** | **0** ✓ | 1 msg (`LASTEXITCODE=...:`) | **✓ A=2 em todos** |

### Veredito final (atualizado — Rodada 2 completa)

**C_NULL é estritamente melhor que Cpoly** para bash B/C (0 ruído vs. 1) sem perder correctness.

```
$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard git-branch; LASTEXITCODE=$((2*!!$?)); exit $LASTEXITCODE
```

| Shell | A | B | C | Ruído B/C |
|---|---|---|---|---|
| macOS sh | 2 ✓ | 2 ✓ | 0 ✓ | **0** (só REASON em B) |
| macOS bash | 2 ✓ | 2 ✓ | 0 ✓ | **0** (só REASON em B) |
| Git Bash não-login | 2 ✓ | 2 ✓ | 0 ✓ | **0** (só REASON em B) |
| PS 5.1 | 2 ✓ | 2 ✓ | 0 ✓ | 1 msg (`LASTEXITCODE=...:CommandNotFound`) |

**Ruído residual**:
- bash/sh: `trackfw: not found` em A (1 msg, irreducível — o binário não está no PATH)
- PS: `LASTEXITCODE=$((2*!!$?)) : O termo '...' não é reconhecido...` em todos os cenários (A, B, C) — idêntico ao Cpoly

**Melhoria sobre Cpoly**: elimina `=2: command not found` do bash B e C (1 msg/cenário). O `${null-/dev/null}` silencia a semente em bash via `/dev/null` e não cria ruído em PS (resolve para null-redirect).

**Ruído de A não é eliminável**: `trackfw: command not found` em bash A é gerado pelo próprio trackfw ausente, não pela semente — nenhuma variante de semente resolve isso.

**Ressalva de correctness PS (herdada do Cpoly)**: se trackfw sair com exit 1 (erro, não deny), C_NULL sai 1 em PS (fail-open para CLIs com critério exit-2). C5 é mais estrito para esse caso.

### Artifacts desta rodada

- Binário macOS: `go build -o <scratchpad>/ml6a2/trackfw ./cmd/trackfw` → trackfw 9.3.1
- Binário Windows: `GOOS=windows GOARCH=arm64 go build -o <scratchpad>/ml6a2/trackfw.exe ./cmd/trackfw` → trackfw 9.3.1
- Scripts de teste Windows: `test_eap2.bat` (R_EAP A/B/C + Cpoly baseline), `test_eap.bat` (descartado — PS sem caminho completo), `cnull_test.sh` (C_NULL Git Bash A), `probes.bat` (probes PS para ${null-/dev/null} e EAP+redirect)
- Pasta limpa: `C:\Users\Lab\ml6a2` apagada ao fim desta rodada
