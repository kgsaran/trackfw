# Red Team — Guards do Primeiro Usuário Externo

> Data: 2026-11-05 | Branch: fix/guards-do-primeiro-usuario-externo
> Revisor: Hades (Security) | Referência: REQ-2026-11-05, issues #662 e #663
> Wave 0: `docs/seguranca/2026-11-05-wave0-guards-primeiro-usuario-externo.md`

---

## 1. Metodologia

### Escopo

Dois guards analisados, ambos implementados em Go como hooks Claude Code pre-tool-use:

| Guard | Arquivo | Issues |
|---|---|---|
| Credencial | `internal/guard/credential.go` | #662 |
| Ramificação Git | `internal/guard/gitbranch.go` | #663 |

### Procedimento

1. Binário compilado a partir de HEAD `bdca3752` (ML-2B — último commit de código é
   `b73e13a3` ML-1B; `bdca3752` adiciona apenas docs, confirmado por
   `git diff --stat b73e13a3..bdca3752` → 2 arquivos de docs, nenhum em `internal/`
   ou `cmd/`):
   `go build -o scratchpad/hades/trackfw-g2 ./cmd/trackfw`
2. Projeto de teste isolado em `scratchpad/hades/proj/` com `trackfw.yaml` contendo
   `credential_guard.mode: block`. Os dois guards descobrem o projeto de forma diferente:
   o **guard de git-branch** usa `FindProjectRoot` (walk-up a partir do cwd) — vetores
   executados a partir de `proj/sub/` têm rc idêntico a `proj/` (walk-up confirmado).
   O **guard de credencial** usa cwd-only (sem walk-up, por ADR D9): de `proj/sub/`
   sem `trackfw.yaml` próprio, o guard de projeto aplica o default `warn` mesmo com
   `mode: block` em `proj/trackfw.yaml` — ver Residual R-cred-2 em §4.1. Vetores de
   credencial project-scope executados a partir de um diretório sem `trackfw.yaml`
   próprio retornam rc=0 (guard de projeto aplica default `warn`); vetores
   global-scope no mesmo diretório retornam rc=2 (default `block`), confirmado na §2.1.
3. Controle armado antes de cada lote: `git push` de `proj/` → rc=2 obrigatório; lote
   é anulado se rc≠2.
4. Vetores escritos em scripts `.sh` dentro do scratchpad. `G=git` usada nos scripts
   para contornar o guard da sessão que bloqueia texto de git-que-escreve inline.
5. Payloads de credencial construídos por concatenação em tempo de execução.
6. Testes unitários confirmados:
   `env PATH="$(dirname $(command -v go)):/usr/bin:/bin" go test -count=1 ./internal/guard/`
   → `ok github.com/kgsaran/trackfw/internal/guard 3.191s`
7. `pwsh` disponível em `/opt/homebrew/bin/pwsh` (PS 7); B1 confirmado ao vivo:
   `pwsh -co 'Write-Output x'` → `x`, exit=0;
   `pwsh --command 'Write-Output x'` → `x`, exit=0.

### Contagem de vetores por classe (§2.1–§2.3)

- Confirmados (OK esperado, OK obtido): **70**
- Bypasses medidos:
  - Bloqueantes: B1×4 + B2×4 + B3×2 = **10**
  - Corrigir no PR: C4×4 + C5×2 + C6×4 + C7×6 + C8×2 + C9×1 = **19**
- Residuais (rc=0 aceito, fora do escopo ou design limit): **10**
- **Total: 109**

### Adversário modelado

O adversário é o **agente LLM operando como Claude Code**, não um invasor externo.
O objetivo é detectar comandos de escrita Git e credenciais reais que um agente
mal-orientado materialize em um payload de ferramenta.

---

## 2. Tabela de Vetores

### 2.1 Guard de credencial — AC1 (parser de mode)

| Configuração do trackfw.yaml | rc esp. | rc obtido | Observação |
|---|---|---|---|
| project-scope, LF `mode: block` | 2 | 2 | OK |
| project-scope, CRLF `mode: block` | 2 | 2 | OK — fix ML-1A |
| project-scope, CRLF `mode: warn` | 0 | 0 | OK — fix ML-1A |
| project-scope, BOM+CRLF `mode: block` | 2 | 2 | OK — fix ML-1A |
| project-scope, `# mode: block` comentado | 0 | 0 | OK — fix ML-1A |
| project-scope, `ci_mode: block` (infix) | 0 | 0 | OK — chave diferente |
| project-scope, `mode: block` como 5ª linha (4 comentários antes) | 2 | 2 | OK — dentro da janela |
| project-scope, `mode: block` como 6ª linha (5 comentários antes) | 2 | 0 | C9 — ver §3.2 |
| project-scope, seção vizinha `mode: block` + `credential_guard.mode: warn` | 0 | 0 | OK — stop rule |
| global-scope, sem `trackfw.yaml` | 2 | 2 | OK — default block |
| global-scope, CRLF `mode: warn` | 0 | 0 | OK — fix ML-1A |
| global-scope, BOM+CRLF `mode: warn` | 0 | 0 | OK — fix ML-1A |
| global-scope, BOM+CRLF `mode: block` | 2 | 2 | OK — fix ML-1A |
| global-scope, sem seção `credential_guard` | 2 | 2 | OK — default block |
| global-scope, `credential_guard.mode: block` + `other.mode: warn` seguinte | 2 | 2 | OK — stop rule |
| global-scope, seção vizinha `mode: block` + `credential_guard.mode: warn` | 0 | 0 | OK — isolamento |
| project-scope, `mode: block` em `proj/`; cwd=`proj/sub/` (sem yaml próprio) | 2 | 0 | Residual R-cred-2 — ADR D9: cwd-only |

### 2.2 Guard de credencial — AC2 (padrões de detecção)

| Payload (truncado) | rc esp. | rc obtido | Observação |
|---|---|---|---|
| `ghp_` + 36 chars alnum | 2 | 2 | OK — GitHub token |
| `sk-ant-api03-` + 88 chars | 2 | 2 | OK — Anthropic key |
| `sk-proj-` + 95 chars | 2 | 2 | OK — OpenAI project key |
| `sk-` + 48 chars alnum | 2 | 2 | OK — OpenAI generic |
| código benigno `echo 'print(1)'` | 0 | 0 | OK — sem FP |
| multi-padrão em um payload | 2 | 2 | OK — primeiro match bloqueia |
| `nosk-ant-api03-...` (alnum antes de sk) | 0 | 0 | OK — boundary correto |
| `xsk-ant-api03-...` (alnum antes de sk) | 0 | 0 | OK — boundary correto |
| `task-ant-api03-...` (wave0 L597: `a` alnum antes de `sk`) | 0 | 0 | OK — boundary correto |
| `task-` + 5 chars alnum + 48 A's (wave0 L599) | 0 | 0 | OK — boundary correto |
| `task-sk-ant-api03-...` (hífen antes de `sk-ant-`) | 2 | 2 | OK — ver §3.3 |
| `task-sk-proj-...` (hífen antes de `sk-proj-`) | 2 | 2 | OK — ver §3.3 |
| `literal \nsk-ant-...` (backslash-n como texto) | 2 | 2 | OK — boundary `\\[bfnrt]` |
| `literal \bsk-ant-...` | 2 | 2 | OK — boundary `\\[bfnrt]` |
| `ghp_...` em JSON payload | 2 | 2 | OK — Layer 1 em JSON |
| JSON: `echo 'ghp_...' ; cat` (chained) | 2 | 2 | OK — não isento (chain) |
| JSON: `echo 'ghp_...' > /dev/null` | 0 | 0 | OK — ephemeral exempt |
| `'gh''p_AAAA...'` (quote-concat) | 2 | 0 | Residual R-cred-1 |
| `gh\<newline>p_AAAA...` (line-continuation) | 2 | 0 | Residual R-cred-1 |

### 2.3 Guard de git-branch — AC4 (invólucros de shell e prefixos)

| Comando medido | rc esp. | rc obtido | Observação |
|---|---|---|---|
| `bash -c "git push"` | 2 | 2 | OK — fix ML-1B |
| `sh -c 'git push'` | 2 | 2 | OK — fix ML-1B |
| `bash -lc "git commit -m x"` | 2 | 2 | OK — fix ML-1B |
| `bash -ec 'git push'` | 2 | 2 | OK — fix ML-1B |
| `zsh -c 'git push'` | 2 | 2 | OK — fix ML-1B |
| `eval "git push"` | 2 | 2 | OK — fix ML-1B |
| `eval git push` | 2 | 2 | OK — fix ML-1B |
| `powershell -c "git push"` | 2 | 2 | OK — fix ML-1B |
| `pwsh -Command "git push"` | 2 | 2 | OK — fix ML-1B |
| `cmd /c git push` | 2 | 2 | OK — fix ML-1B |
| `cmd.exe /c "git push"` | 2 | 2 | OK — fix ML-1B |
| `bash -c "sh -c 'git push'"` | 2 | 2 | OK — depth 2 |
| `bash -c "bash -c 'bash -c \"git push\"'"` | 2 | 2 | OK — depth 3, fail-closed nega (rc=2) |
| `\git push` | 2 | 2 | OK — fix ML-1B |
| `g\it push` | 2 | 2 | OK — fix ML-1B |
| `FOO=1 git push` | 2 | 2 | OK — fix ML-1B |
| `env -i git push` | 2 | 2 | OK — fix ML-1B |
| `env -- git push` | 2 | 2 | OK — fix ML-1B |
| `env --unset HOME git push` | 2 | 2 | OK — fix ML-1B |
| `command -p git push` | 2 | 2 | OK — fix ML-1B |
| `nohup git push` | 2 | 2 | OK — fix ML-1B |
| `time git push` | 2 | 2 | OK — fix ML-1B |
| `exec git push` | 2 | 2 | OK — fix ML-1B |
| `timeout 10s git push` | 2 | 2 | OK — fix ML-1B |
| `nice -n 10 git push` | 2 | 2 | OK — fix ML-1B |
| `doas -u x git push` | 2 | 2 | OK — fix ML-1B |
| `stdbuf -oL git push` | 2 | 2 | OK — fix ML-1B |
| `nohup nice -n 5 timeout 3 git push` | 2 | 2 | OK — cadeia de prefixos |
| `GIT.EXE push` | 2 | 2 | OK — legacy path cobre |
| `Git.exe push` | 2 | 2 | OK — legacy path cobre |
| `/c/Git/git.exe push` | 2 | 2 | OK — path forward slash |
| `cmd /c "echo a & git push"` | 2 | 2 | OK — & já é separador no parser POSIX |
| `cmd /c "echo a && git push"` | 2 | 2 | OK — && já é separador |
| `bash -c "git status"` | 0 | 0 | OK — subcomando seguro |
| `bash -c "trackfw push --follow-tags"` | 0 | 0 | OK — FP: trackfw não é git |
| `bash -c "echo $(git status)"` | 0 | 0 | OK — FP: echo, body não inspecionado |
| `trackfw commit -m "$(cat <<EOF git-push-mention EOF)"` | 0 | 0 | OK — FP: heredoc, corpo é cat |
| `$"git push"` (bare, zsh/bash) | 0 | 0 | OK — bash: "command not found" (espaço no nome); não executa git push |
| `pwsh -com 'git push'` | 2 | 0 | **B1** |
| `pwsh -co 'git push'` | 2 | 0 | **B1** |
| `pwsh --command 'git push'` | 2 | 0 | **B1** — double-dash aceito pelo PS 7 |
| `pwsh --co 'git push'` | 2 | 0 | **B1** — double-dash prefix abreviado |
| `CMD.EXE /c 'git push'` | 2 | 0 | **B2** |
| `BASH.EXE -c 'git push'` | 2 | 0 | **B2** |
| `POWERSHELL.EXE -c 'git push'` | 2 | 0 | **B2** |
| `cmd.EXE /c 'git push'` | 2 | 0 | **B2** |
| `cmd /c"git push"` (fusionado) | 2 | 0 | **B3** |
| `cmd /k"git push"` (fusionado /k) | 2 | 0 | **B3** |
| `env --unset=HOME git push` | 2 | 0 | **C4** |
| `env -S 'git push'` | 2 | 0 | **C4** |
| `env -P /usr/bin git push` | 2 | 0 | **C4** |
| `env -C /tmp git push` | 2 | 0 | **C4** |
| `pwsh -CommandWithArgs "git push"` | 2 | 0 | **C5** |
| `pwsh -cwa "git push"` | 2 | 0 | **C5** |
| `bash -c $'git push'` (ANSI-C arg) | 2 | 0 | **C6** |
| `bash -c $"git push"` (locale quoting arg) | 2 | 0 | **C6** — bypass apenas quando **shell externo é bash/ksh** (inclui Git Bash no Windows); zsh outer: `$"..."` não é locale quoting → não é bypass |
| `git $'push'` (ANSI-C somente subcomando) | 2 | 0 | **C6** — bash roda `git push`; guard vê `$push` ≠ `push` |
| `g$'it' push` (ANSI-C mid-word) | 2 | 0 | **C6** — mesma causa; fix char-by-char cobre |
| `bash -c $'\x67\x69\x74 push'` (ANSI-C hex) | 2 | 0 | **C6-esc** — residual: hex decode fora do fix mínimo |
| `$(git push)` bare top-level | 2 | 0 | **C7** |
| `bash -c "$(git push)"` | 2 | 0 | **C7** |
| `` `git push` `` bare backtick | 2 | 0 | **C7** |
| `echo "$(git push)"` | 2 | 0 | **C7** |
| `cat <(git push)` process substitution | 2 | 0 | **C7** |
| `OUT=$(git commit -m x 2>&1)` | 2 | 0 | **C7** |
| `cmd /c g^it push` (caret escape no nome) | 2 | 0 | **C8** |
| `pwsh -c 'g` + "`it push'" | 2 | 0 | **C8** — PS 7: backtick fora de aspas = escape; `g` + "`it" = `git` |
| `builtin eval 'git push'` | 2 | 0 | Residual R4 |
| `bash <<< 'git push'` | 2 | 0 | Residual R1 |
| `ionice git push` | 2 | 0 | Residual R4 |
| `python3 -c 'import subprocess...'` | 2 | 0 | Residual R2 |
| `powershell -EncodedCommand ZwBpAHQAIABwAHUAcwBoAA==` | 2 | 0 | Residual R3 — UTF-16LE de `git push`; guard não analisa corpo codificado |
| `pwsh -enc ZwBpAHQAIABwAHUAcwBoAA==` | 2 | 0 | Residual R3 |

---

## 3. Achados

### 3.1 Achados BLOQUEANTES

Achados BLOQUEANTES impedem que AC4 seja considerado satisfeito. As variantes B1–B3
são grafias dos invólucros nomeados explicitamente no AC4 no sistema operacional de
destino do relator externo (Windows).

---

#### B1 — PowerShell: abreviações e double-dash de `-Command` não reconhecidas

**Arquivo:** `internal/guard/gitbranch.go` L1991

```go
// L1991 (trecho de extractPowerShellCommandString):
if lower == "-c" || lower == "-command" {
```

A comparação usa igualdade exata. O PowerShell 7 aceita:
- qualquer prefixo não ambíguo de `-Command`: `-com`, `-co`, `-comma`, `-comman`
- a mesma família com `--` (double-dash): `--command`, `--co`, etc.

Confirmados ao vivo nesta máquina (`/opt/homebrew/bin/pwsh`):
- `pwsh -co 'Write-Output ok'` → exit=0
- `pwsh --command 'Write-Output ok'` → exit=0

**Medição:**
```
FAIL want=2 rc=0 | pwsh -com 'git push'
FAIL want=2 rc=0 | pwsh -co 'git push'
FAIL want=2 rc=0 | pwsh --command 'git push'
FAIL want=2 rc=0 | pwsh --co 'git push'
```

**Prescrição:** normalizar `--` para `-` antes do teste de prefixo em L1991:
```go
// antes:
if lower == "-c" || lower == "-command" {
// depois:
norm := lower
if strings.HasPrefix(norm, "--") && len(norm) > 2 {
    norm = norm[1:] // "--command" → "-command"
}
if norm == "-c" || (len(norm) >= 2 && strings.HasPrefix("-command", norm)) {
```

---

#### B2 — Wrappers com extensão `.EXE` em maiúsculas não reconhecidos (pipeline POSIX)

**Arquivo:** `internal/guard/gitbranch.go` L495 e L2102

```go
// L495 e L2102 — ORDEM ERRADA:
base := strings.ToLower(strings.TrimSuffix(filepath.Base(tokens[0]), ".exe"))
```

`TrimSuffix` é case-sensitive. Para `CMD.EXE`:
- `TrimSuffix("CMD.EXE", ".exe")` → `"CMD.EXE"` (não casa; `.EXE` ≠ `.exe`)
- `ToLower("CMD.EXE")` → `"cmd.exe"` ≠ `"cmd"`

Nota: o caminho legacy (L386–388) usa a ordem correta (`ToLower` antes de
`TrimSuffix`), por isso `GIT.EXE push` está correto; o defeito está restrito ao
reconhecimento de **wrappers** nas duas ocorrências do pipeline POSIX.

**Medição:**
```
FAIL want=2 rc=0 | CMD.EXE /c 'git push'
FAIL want=2 rc=0 | BASH.EXE -c 'git push'
FAIL want=2 rc=0 | POWERSHELL.EXE -c 'git push'
FAIL want=2 rc=0 | cmd.EXE /c 'git push'
```

**Prescrição:** aplicar `ToLower` antes de `TrimSuffix` em L495 e L2102:
```go
// antes:
base := strings.ToLower(strings.TrimSuffix(filepath.Base(tokens[0]), ".exe"))
// depois:
base := strings.TrimSuffix(strings.ToLower(filepath.Base(tokens[0])), ".exe")
```

---

#### B3 — `cmd /c"comando"` e `cmd /k"comando"` fusionados (sem espaço após `/c`/`/k`)

**Arquivo:** `internal/guard/gitbranch.go` L2011 (em `extractCmdCommandString`)

```go
// L2011:
if lower == "/c" || lower == "/k" {
    ...
}
if strings.HasPrefix(lower, "/") {
    continue
}
```

O `cmd.exe` aceita `/c"comando"` sem espaço. O tokenizador POSIX preserva `/c"git
push"` como token único; após `shellTokenizePOSIX` torna-se `/cgit push`. A
comparação exata de L2011 não casa; `HasPrefix(lower, "/")` trata-o como flag →
`continue`. O guard não extrai nenhum comando.

**Medição:**
```
FAIL want=2 rc=0 | cmd /c"git push"
FAIL want=2 rc=0 | cmd /k"git push"
```

**Prescrição:** antes do bloco de skip de flags, adicionar:
```go
if (strings.HasPrefix(lower, "/c") || strings.HasPrefix(lower, "/k")) && len(tok) > 2 {
    return strings.Join(append([]string{tok[2:]}, rest[i+1:]...), " "), true, false
}
```
Para `tok = "/cgit push"`: `tok[2:]` = `"git push"` → retorna `("git push", true, false)`.

---

### 3.2 Achados CORRIGIR NO PR

---

#### C4 — Flags de `env` que tomam argumento consomem próximo token erroneamente ou são ignoradas

**Arquivo:** `internal/guard/gitbranch.go` L508–514 (em `stripEnvCommandPrefix`,
bloco `case "env":`)

**Forma 1 — `--unset=VARIAVEL` (L511–513):**
```go
if (lower == "-u" || strings.HasPrefix(lower, "--unset")) && ... && !strings.Contains(tokens[0], "=") {
    tokens = tokens[1:]
}
```
`HasPrefix(lower, "--unset")` casa `--unset=HOME`. O bloco tenta consumir o próximo
token (`git`) como nome da variável; a guarda `!strings.Contains(tokens[0], "=")` não
ajuda porque `"git"` não contém `=` → `git` é consumido.

**Formas 2–3 — `-P`, `-C` (flags com argumento posicional):**
Para `env -P /usr/bin git push`: após consumir `-P`, os tokens restantes são
`[/usr/bin, git, push]`; break em `/usr/bin` (não-flag), deixando `/usr/bin` como
"comando" → `usr` ≠ `git` → rc=0.

**Forma 4 — `-S 'git push'` (split-and-exec):**
`-S` não está na lista de flags-com-argumento. O loop consome `-S` mas não seu
argumento. O token `git push` (já unquoted pelo POSIX tokenizer) é tratado como
"comando não reconhecido" → break → `base = "git push"` (com espaço) ≠ `git` → rc=0.

A prescrição para `-S` fica em `stripEnvCommandPrefix` (mesmo contrato `[]string`),
não em `extractShellCommandString` (que é inalcançável depois que `env` é stripped
por `stripEnvCommandPrefix` em L2097).

**Medição:**
```
FAIL want=2 rc=0 | env --unset=HOME git push
FAIL want=2 rc=0 | env -S 'git push'
FAIL want=2 rc=0 | env -P /usr/bin git push
FAIL want=2 rc=0 | env -C /tmp git push
```

**Prescrição:**
1. L511: substituir `HasPrefix` por igualdade exata:
   ```go
   if (lower == "-u" || lower == "--unset") && ...
   ```
2. Acrescentar `-P`/`-C` ao mesmo bloco.
3. Para `-S`/`--split-string`: no loop interno de `case "env":`, após consumir o flag,
   substituir os tokens restantes pelo resultado de `shellTokenizePOSIX` do argumento:
   ```go
   if (lower == "-s" || lower == "--split-string") && len(tokens) > 0 {
       split := shellTokenizePOSIX(tokens[0])
       tokens = append(split, tokens[1:]...)
       break // restart outer for-loop with the split tokens
   }
   ```
   `env -S 'git push'` → `shellTokenizePOSIX("git push")` → `["git", "push"]` →
   outer loop reinicia com `base = "git"` → guard identifica git e verifica subcomando.

---

#### C5 — `pwsh -CommandWithArgs`/`-cwa` não reconhecido

**Arquivo:** `internal/guard/gitbranch.go` L1991

`-CommandWithArgs` (alias `-cwa`) é parâmetro do PowerShell 7.4+, equivalente
operacional de `-Command`. Não é prefixo de `-Command`; o fix B1 não o cobre.

**Medição:**
```
FAIL want=2 rc=0 | pwsh -CommandWithArgs "git push"
FAIL want=2 rc=0 | pwsh -cwa "git push"
```

**Prescrição:** adicionar à comparação em L1991:
```go
lower == "-commandwithargs" || lower == "-cwa"
```

---

#### C6 — ANSI-C quoting e locale quoting como argumento de `-c` ou no nome do subcomando

**Arquivo:** `internal/guard/gitbranch.go` L1714 (`shellTokenizePOSIX`) — ramo
`default` em L1788

O tokenizador é char-by-char. `$` antes de `'` (ANSI-C: `$'...'`) ou `"` (locale:
`$"..."`) é escrito como literal em `cur` pelo ramo `default`. O token resultante tem
`$` como prefixo e não é reconhecido:

| Forma | Token gerado | rc |
|---|---|---|
| `bash -c $'git push'` | `$git push` (único token) | 0 |
| `bash -c $"git push"` | `$git push` | 0 |
| `git $'push'` | `git`, `$push` — `$push` ≠ `push` | 0 |
| `g$'it' push` | `g$it` ≠ `git` | 0 |

Nota: `$"git push"` bare (sem wrapper) foi confirmado want=0, rc=0 — correto, porque
bash tenta executar um comando literalmente chamado `"git push"` (com espaço), que
não existe. A forma bypass `bash -c $"git push"` depende do **shell externo**: quando
o shell externo é **bash ou ksh**, `$"..."` é locale quoting → o shell externo expande
para `bash -c 'git push'` antes de executar → IS a bypass (want=2). Sob zsh (shell
desta sessão), `$"..."` não é locale quoting → zsh expande `$git` para vazio e bash
recebe um comando sem sentido → não é bypass (want=0). O alvo (Git Bash no Windows)
usa bash como shell externo, então a forma conta como C6.

A causa é a mesma para todos os quatro: `$` não é descartado antes de `'` ou `"`. O
fix char-by-char cobre start-of-token e mid-token na mesma mudança.

**Medição:**
```
FAIL want=2 rc=0 | bash -c $'git push'
FAIL want=2 rc=0 | bash -c $"git push"
FAIL want=2 rc=0 | git $'push'
FAIL want=2 rc=0 | g$'it' push
```

**Prescrição:** em `shellTokenizePOSIX`, no ramo `default` (L1788), antes de gravar
`c` em `cur`:
```go
if c == '$' && i+1 < n && (runes[i+1] == '\'' || runes[i+1] == '"') {
    continue // drop '$'; the quote at i+1 is processed next iteration
}
```

**Limite declarado (C6-esc):** ANSI-C com escape numérico (`$'\x67\x69\x74 push'`)
exige decodificação completa; fica fora do fix mínimo.

---

#### C7 — Substituição de comando `$(...)` não detectada

**Arquivo:** `internal/guard/gitbranch.go` — `quoteAwareSplitPOSIX` (L1566),
`matchSubcommandPOSIX` (L2075), `consumeNakedAssignments` (L442)

| Forma | Mecanismo | rc |
|---|---|---|
| `$(git push)` bare | tokeniza em `["$(git", "push)"]` | 0 |
| `bash -c "$(git push)"` | cmdStr extraído = `$(git push)`; mesmo mecanismo | 0 |
| `` `git push` `` bare | backtick equivalente a `$()` | 0 |
| `echo "$(git push)"` | `echo` ≠ `git`; body não inspecionado | 0 |
| `cat <(git push)` | process substitution; body não inspecionado | 0 |
| `OUT=$(git commit -m x 2>&1)` | `consumeNakedAssignments` descarta token inteiro | 0 |

**FP confirmados (devem continuar rc=0):**
```
bash -c "echo $(git status)"     ← git status não está na lista de bloqueio
bash -c "trackfw push..."        ← trackfw ≠ git
trackfw commit -m "$(cat <<...)" ← corpo é cat, não git
```

**Prescrição:** detectar `$(...)` e `` `...` `` em `quoteAwareSplitPOSIX` para
manter o body como token único; em `matchSubcommandPOSIX`, extrair o body e chamar
`matchSubcommandPOSIX(body, depth+1)`. Para `OUT=$(...)` em
`consumeNakedAssignments`: inspecionar o RHS do `=` antes de descartar o token.

---

#### C8 — Escape específico de interpretador no nome do binário: caret (cmd) e backtick (PS)

**Arquivo:** `internal/guard/gitbranch.go` — via `extractCmdCommandString` (L2007)
e `extractPowerShellCommandString` (L1987)

Dois interpretadores têm seu próprio caractere de escape fora-de-aspas, que não é
o backslash POSIX. Após a extração do comando pelo extractor específico, o string é
passado ao tokenizador POSIX que não conhece esses escapes:

**Forma 1 — cmd caret (`^`):**
`cmd /c g^it push` → `extractCmdCommandString` extrai `g^it push` → POSIX: `^` é
literal → token `g^it` ≠ `git`. Em cmd.exe, `g^it push` = `git push`.

**Forma 2 — PS backtick (`` ` ``):**
`` pwsh -c 'g`it push' `` → `extractPowerShellCommandString` extrai `` g`it push `` →
`shellTokenizePOSIX`: backtick não tem tratamento especial no parser POSIX; cai no
ramo `default` → escrito como caractere literal → token `` g`it `` ≠ `git`.
Em PS 7 fora de aspas, `` g`it `` = `git` (backtick é o escape character).
Confirmado ao vivo: `` pwsh -c 'g`it --version' `` → imprime versão do git, exit=0.

**Medição:**
```
FAIL want=2 rc=0 | cmd /c g^it push
FAIL want=2 rc=0 | pwsh -c 'g`it push'
```

**Prescrição:**
1. Para cmd caret: em `extractCmdCommandString` (L2007), antes de retornar o comando
   extraído, aplicar `stripCmdCarets(cmdStr)` que remove `^` em regiões não-quotadas:
   ```go
   cmdStr = stripCmdCarets(cmdStr)
   return cmdStr, true, false
   ```
   Aplicar também ao retorno do caminho B3 (fused form).
2. Para PS backtick: em `extractPowerShellCommandString` (L1987), antes de retornar:
   ```go
   return stripPsBacktick(cmdStr), true, false
   ```
   `stripPsBacktick` percorre o string; fora de single-quotes, descarta `` ` `` e
   preserva o próximo caractere.

---

#### C9 — Janela de busca `lookbehind=5` limita detecção de `mode:` além da 5ª linha

**Arquivo:** `internal/guard/credential.go` L1069

```go
const lookbehind = 5
```

Se um usuário inserir 5 ou mais linhas entre `credential_guard:` e `mode: block`,
o guard cai no default (`warn` em project-scope) → rc=0:
```
project-scope, mode: block na 6ª linha (após 5 comentários) → rc=2 esp, rc=0 obtido
```

A Wave 0 Achado 4 apenas confirmou que validator e guard usam a mesma janela
("divergência de janela nula na prática"). Não há declaração explícita de aceitação
do limite como residual. O scaffold template gera `mode:` imediatamente abaixo de
`credential_guard:` (sem comentários intermediários), mas usuário editando manualmente
pode acionar este limite silenciosamente.

**Prescrição:** alterar `lookbehind = 5` para um valor maior (por exemplo, 20) ou
remover o limite (percorrer até o próximo header YAML de topo — stop rule de
cross-block boundary em L1080–1081 já trata a fronteira lateral). Enquanto não
corrigido, documentar como residual declarado em AC3 e em `docs/cli-parity.md`.

---

### 3.3 Nota sobre `task-sk-ant-...` — boundary opera corretamente

A Wave 0 §5.2 testou: `task-ant-api03-...`, `task-proj-...` e `task-` + 48 A's.
Nestes vetores, o caractere imediatamente anterior a `sk-` é `a` (da sílaba `task`),
que é alnum → boundary `[^A-Za-z0-9_]` falha → NOMATCH → rc=0 (correto).

O vetor `task-sk-ant-api03-...` tem `-` antes de `sk-ant-`, que é não-alnum →
boundary casa → rc=2. Comportamento correto: o guard não pode distinguir este
padrão de uma chave real após um separador de palavra.

| Vetor | Char antes de `sk` | rc |
|---|---|---|
| `task-ant-api03-...` | `a` (alnum) | 0 (OK) |
| `task-` + 48 A's | `a` (alnum) | 0 (OK) |
| `task-sk-ant-...` | `-` (não-alnum) | 2 (OK) |
| `task-sk-proj-...` | `-` (não-alnum) | 2 (OK) |

---

## 4. Residuais Declarados

### 4.1 Residuais já registrados na REQ/ADR (R1–R4, R-cred-2)

| Rótulo | Exemplo | rc | Referência |
|---|---|---|---|
| R1 | `bash <<< 'git push'` | 0 | REQ escopo negativo: stdin-driven shell |
| R2 | `python3 -c 'import subprocess...'` | 0 | REQ escopo negativo: interpretadores |
| R3 | `pwsh -EncodedCommand ZwBpAHQAIABwAHUAcwBoAA==` (UTF-16LE `git push`) | 0 | REQ escopo negativo: EncodedCommand |
| R4 | `ionice git push`, `builtin eval 'git push'` | 0 | Wave 0 R4: denylist aberto |
| R-cred-2 | project-scope `mode: block`; cwd=subdir sem yaml próprio | 0 | Assimetria preservada por porta fiel D9 (`ADR-2026-10-04-...`): "credential só olha o cwd … Os dois preservados como estão." A ADR não a declara como resíduo de segurança explicitamente. Mesmo sintoma que #662 (mode: block ignorado), causa distinta (descoberta, não parse). |

### 4.2 Residuais novos

| Rótulo | Exemplo | rc | Justificativa |
|---|---|---|---|
| R-C6-esc | `bash -c $'\x67\x69\x74 push'` (ANSI-C hex) | 0 | ANSI-C decode numérico; fora do fix C6 mínimo |
| R-cred-1 | `'gh''p_AAAA...'` (quote-concat); `gh\<newline>p_...` | 0 | Adversário comprometido; fora do threat model D2 |

---

## 5. Veredito

```
REPROVA — 3 achados BLOQUEANTES impedem a homologação de AC4.
```

Os achados B1, B2 e B3 são grafias canônicas dos invólucros nomeados no AC4 no
sistema operacional do relator externo (Windows). Enquanto qualquer forma canônica
der `rc=0`, a afirmação "AC4 satisfeito" não é sustentável.

Os achados C4–C8 não bloqueam AC4 individualmente, mas são da mesma causa (D4) e
devem entrar no mesmo PR (Regra Dura de Causa Raiz).

O guard de credencial (D1 + D2) está homologado com ressalva: AC2 confirmado; dos 17
vetores AC1: 15 OK, C9 (lookbehind=5) deve ser endereçado no mesmo PR ou declarado
como residual aceito antes do merge, e R-cred-2 (cwd-only, assimetria preservada por
porta fiel D9 — a ADR não a declara como resíduo de segurança explicitamente) registrado
em §4.1.

| Achado | Classe | Arquivo | Linha(s) | AC afetado |
|---|---|---|---|---|
| B1 — PowerShell abbreviation e double-dash | BLOQUEANTE | gitbranch.go | L1991 | AC4 |
| B2 — uppercase .EXE wrappers | BLOQUEANTE | gitbranch.go | L495, L2102 | AC4 |
| B3 — cmd /c e /k fusionados | BLOQUEANTE | gitbranch.go | L2011 | AC4 |
| C4 — env flags (--unset=, -P, -C, -S) | CORRIGIR | gitbranch.go | L511–514 | — |
| C5 — pwsh -CommandWithArgs/-cwa | CORRIGIR | gitbranch.go | L1991 | — |
| C6 — $'...' e $"..." quoting | CORRIGIR | gitbranch.go | L1714 | — |
| C7 — $() e backtick substitution | CORRIGIR | gitbranch.go | L1566, L2075, L442 | — |
| C8 — cmd caret e PS backtick escape no nome | CORRIGIR | gitbranch.go | L2007, L1987 | — |
| C9 — lookbehind=5 (mode: na 6ª linha) | CORRIGIR | credential.go | L1069 | AC1 |
