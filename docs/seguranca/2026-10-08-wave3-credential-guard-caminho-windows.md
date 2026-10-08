---
title: "Threat Model: Forma de caminho Windows na 2ª camada do credential guard"
date: 2026-10-08
author: hades-tf
roadmap: ROADMAP-2026-10-06-*.md
ml: ML-3A
req: REQ-2026-10-06
issue: "#544"
status: Entregue
---

# Threat Model — ML-3A: Forma de caminho Windows na 2ª camada do credential guard

> REQ-2026-10-06 · Wave 3 · 2026-10-08 · hades-tf

## 1. Enumeração de Sítios

### 1.1 Todos os sítios da 2ª camada que interpretam caminho

**Sítio 1 — `credResolveArg` (credential.go:499–514) — argumento de cat/head/tail/jq/grep**

Chamado em dois pontos de `credSecondLayer`:
- Linha 540: `credSecondLayer` — Layer 2a — alvo de redirecionamento: `credScanFile(credResolveArg(target, shellCwd))`
- Linha 566: `credSecondLayer` — Layer 2b — argumento de comando: `credScanFile(credResolveArg(tok, shellCwd))`
- `credDeepScan` (linha 712) chama `credSecondLayer` via linha 762 — mesmo caminho
- `credNonJSONLayerTwoB` (linha 785) chama `credSecondLayer` passando `nil` para redirectMatches — apenas Layer 2b

Lógica de resolução (linhas 506–513) — dois sub-casos de falha:

```go
clean = strings.TrimRight(clean, "},")
if clean == "" {
    return arg
}
if baseCwd != "" && !filepath.IsAbs(clean) {   // ← sub-caso B
    return filepath.Join(baseCwd, clean)
}
return clean                                    // ← sub-caso A retorna aqui
```

**Sub-caso A (sem `tool_info.cwd` no payload, `baseCwd=""`):** o `if baseCwd != ""` é falso; o caminho Git Bash `/c/Users/...` é devolvido como-está. `credScanFile` chama `os.Stat("/c/Users/...")`. O Windows interpreta `/c/` como `C:\c\` na unidade corrente → o caminho `C:\c\Users\...` não existe → stat falha silenciosamente → não detecta. **O `filepath.IsAbs` nem é consultado neste sub-caso.**

**Sub-caso B (com `tool_info.cwd` nativo no payload, `baseCwd="C:\\..."`):** `filepath.IsAbs("/c/Users/...")` = `false` no Windows → `filepath.Join("C:\\...", "/c/.../s.env")` = `C:\c\...\s.env` (caminho errado) → stat falha → não detecta. Medido: `cwd=C:\\Users\\Lab\\tf-ml3a + arg=/c/Users/Lab/tf-ml3a/s.env` → rc=0 detected=false.

`filepath.IsAbs` no Windows exige letra de unidade ou UNC. Para `/c/Users/...`: retorna `false`.

**Sítio 2 — `credRedirectRe` (credential.go:30) — extração de alvo de redirecionamento**

```go
credRedirectRe = regexp.MustCompile(`[0-9]?>>?[ \t]*[^ \t\r\n|&;,:]+`)
```

O caractere `:` está na classe de exclusão. Para `echo hi > C:\path\s.env`, a correspondência é `> C` (para antes do `:`). O alvo extraído é `C`, não o caminho completo.

**Todos os consumidores de `credRedirectRe` (grep `internal/guard/credential.go`):**

| Linha | Contexto | String sobre a qual consome |
|-------|---------|----------------------------|
| 97 | JSON path — `redirectMatches = credRedirectRe.FindAllString(shellCmd, -1)` | `shellCmd` (JSON-decoded shell command) |
| 101 | non-JSON path — `redirectMatches = credRedirectRe.FindAllString(contextStr, -1)` | `contextStr` (raw payload bytes) |
| 151 | F2 non-JSON: `cmdRedirects := credRedirectRe.FindAllString(cmd, -1)` | cmd extraído por `credCmdLineRe` |
| 207 | `RunCredentialGlobal` (global) — JSON path | `shellCmd` |
| 211 | `RunCredentialGlobal` (global) — non-JSON path | `contextStr` |
| 248 | `RunCredentialGlobal` (global) — F2 non-JSON | cmd extraído por `credCmdLineRe` |
| 761 | `credDeepScan` — walk de JSON aninhado | cmd de cada nó |

Nota: `credNonJSONLayerTwoB` (linha 785) NÃO usa `credRedirectRe` diretamente — passa `nil` para `redirectMatches` ao chamar `credSecondLayer`.

A mudança em `credRedirectRe` (remover `:`) afeta todas as linhas acima. Os consumidores passam as matches para: `credSecondLayer` (Layer 2a scan de arquivo), `credIsAllEphemeral` (F1), `credAllTargetsAreDevNull` (F2/Layer 1 exemption). A F2 exemption (`/dev/null`) já estava extraindo `/dev/null` corretamente antes e depois da mudança (`:` não está em `/dev/null`).

**Sítio 3 — `credGlobToken` (credential.go:298) — expansão de glob**

Chama `filepath.Glob(pattern)` (linha 304). Glob no Windows usa separadores nativos. Para o padrão `/c/Users/...s.*`, `filepath.Glob` tenta no namespace `/c/` que não existe como caminho Windows → nenhuma correspondência → token literal devolvido → `credResolveArg` com o literal `/c/...` → mesmo defeito do Sítio 1.

**Sítio 4 — `credExtractToolInfoCwd` (credential.go:472–490) — cwd do payload**

Extrai `tool_info.cwd` do JSON. O valor é passado como `shellCwd` para `credSecondLayer` e daí para `credResolveArg`. Se o CLI escreve `tool_info.cwd = /c/Users/...` (Git Bash), então para qualquer argumento relativo: `filepath.Join("/c/Users/...", "relative.env")` produz `/c/Users/.../relative.env`, e `os.Stat` falha porque não é um caminho Windows válido.

**Sítio 5 — `credScanFile` (credential.go:369–402) — abertura real do arquivo**

Usa `os.Stat(path)` (linha 382). A abertura do arquivo pela syscall do Windows não reconhece caminhos no formato Git Bash. O sítio em si não interpreta o caminho — é apenas o receptor. O defeito está nos sítios 1 e 2 que fornecem o caminho errado.

**Resumo do grep por `filepath.` / `os.Stat` em `internal/guard/` (pacote completo: credential.go, gitbranch.go, payload.go):**

| Arquivo:linha | Uso | Contexto | Relevante para Layer 2? |
|---------------|-----|---------|------------------------|
| credential.go:73 | `filepath.Join(cwd, "trackfw.yaml")` | Verifica presença do projeto | Não |
| credential.go:160, 263 | `filepath.Join(cwd, "trackfw.yaml")` | Lê modo | Não |
| credential.go:304 | `filepath.Glob(pattern)` | Glob em token | **Sim — Sítio 3** |
| credential.go:309, 313 | `filepath.Base(pattern/m)` | Filtro de dotfiles | Não |
| credential.go:510–511 | `filepath.IsAbs(clean)` + `filepath.Join` | **Sítio principal do defeito — sub-caso B** | **Sim — Sítio 1B** |
| credential.go:513 | `return clean` | Devolve path como-está — sub-caso A | **Sim — Sítio 1A** |
| credential.go:663 | `filepath.Base(fields[0])` | Extrai argv0 para `credIsSimpleCmd` | Não |
| credential.go:950, 958 | `filepath.Join(root, roadmapDir, ...)` | Escreve attention JSON | Não |
| gitbranch.go:76 | `filepath.EvalSymlinks(cwd)` | Resolve cwd para git-branch guard | Fora da Layer 2 do credential guard |
| gitbranch.go:178 | `os.Stat(filepath.Join(dir, "trackfw.yaml"))` | Busca trackfw.yaml | Fora da Layer 2 |
| gitbranch.go:181 | `filepath.Dir(dir)` | Sobe na hierarquia | Fora da Layer 2 |
| payload.go | (nenhum `filepath.`/`os.Stat` relevante) | — | Não |

Todos os sítios que interpretam caminho para fins de credential scan estão em `credential.go`. Os hits em `gitbranch.go` são para o guard de branch, fora do escopo desta Wave.

### 1.2 Como cada sítio interpreta cada forma de caminho

| Forma | `IsAbs` Windows | `IsAbs` POSIX | Comportamento |
|-------|----------------|--------------|---------------|
| `C:\Users\...` (JSON-esc: `C:\\Users\\...`) | `true` | `false` | Windows: absoluto, abre. POSIX: relativo ao cwd |
| `C:/Users/...` | `true` | `false` | Windows: absoluto, abre. POSIX: relativo ao cwd |
| `/c/Users/...` (Git Bash lower) | **`false`** | `true` | Windows: sem cwd → devolvido como-está → `os.Stat` falha (sub-caso A); com cwd nativo → join errado (sub-caso B). POSIX: absoluto, funciona. |
| `/C/Users/...` (Git Bash upper) | **`false`** | `true` | Idem que `/c/` |
| `/cygdrive/c/Users/...` | **`false`** | `true` | Mesmo defeito — sub-casos A e B |
| `\\server\share\...` (UNC) | `true` (Go IsAbs aceita `\\`) | N/A | Windows: absoluto. Acesso depende de permissões de rede |
| `~/...` | `false` | `false` | Relativo em ambos os SOs; join com cwd. `~` não é expandido pelo guard |
| `s.env` (relativo) | `false` | `false` | Join com shellCwd ou cwd do binário |
| `"C:\..."` (com aspas, backslash literal) | N/A — JSON inválido | N/A | Cai no path não-JSON; `credCmdLineRe` falha (para na 1ª aspa) |

## 2. Medição na VM

**Proveniência do binário:** branch `fix/credential-guard-caminho-git-bash-windows`, commit `a3f98e99`, `git diff origin/main --stat -- internal/ cmd/` = vazio (nenhum arquivo de produto alterado nesta branch). Binário: `tf.exe --version` = `trackfw 9.3.3`.

**Ambiente:** Windows 11 ARM64 · Git Bash · cwd `C:\Users\Lab\tf-ml3a` com `trackfw.yaml` presente.

**Leitura de modo:** controlada por `credential_guard.mode:` no `trackfw.yaml`; `TRACKFW_CREDENTIAL_MODE` não é lida pelo binário.

**Arquivo-alvo:** `C:\Users\Lab\tf-ml3a\s.env` com chave `aws_access_key_id = <chave de exemplo>`.

### 2.1 Controles

| Teste | rc | detected | Interpretação |
|-------|-----|---------|---------------|
| CTRL_VAC: `cat C:\\Users\\...\\s.env` (JSON-esc), block/local | 2 | true | Layer 2b varre, binário ativo |
| CTRL_VAC: `cat C:/Users/.../s.env`, block/local | 2 | true | Forward slash funciona |
| CTRL_NEG: `git status`, block/local | 0 | false | Payload inocente não detecta |

### 2.2 Layer 2b — argumento de cat (mode=block, scope=local)

| Forma | rc | detected | Status |
|-------|-----|---------|--------|
| `C:\\Users\\...` (JSON-esc backslash) | 2 | true | ✅ |
| `C:/Users/...` (forward slash) | 2 | true | ✅ |
| `/c/Users/...` (Git Bash lower) | 0 | false | 🔴 **NÃO DETECTA** |
| `/C/Users/...` (Git Bash upper) | 0 | false | 🔴 **NÃO DETECTA** |
| `s.env` (relativo, sem cwd no payload) | 2 | true | ✅ |
| `~/tf-ml3a/s.env` (til) | 0 | false | não detecta (til não expandido) |
| `/cygdrive/c/Users/...` | 0 | false | 🔴 **NÃO DETECTA** |
| `\\\\localhost\\C$\\...` (UNC, JSON-esc) | 2 | true | ✅ detecta quando admin share acessível (VM: C$ habilitado) |
| `/c/.../s.*` (glob, Git Bash) | 0 | false | 🔴 glob falha; `filepath.Glob("/c/...")` não reconhece namespace `/c/` no Windows → nenhuma correspondência → token literal devolvido → BUG-1 sub-caso A em `credResolveArg` |
| `C:/Users/.../s.*` (glob, forward) | 2 | true | ✅ glob expandiu corretamente |

**Layer 2b com mode=warn:** detecção inalterada; rc=0 em vez de 2 para os casos que detectam (comportamento correto do modo warn).

**Layer 2b com scope=global:** padrão idêntico ao local para as formas testadas.

### 2.3 Layer 2b — com tool_info.cwd no payload (mode=block, scope=local)

| Forma do cwd | arg | rc | detected | Status |
|-------------|-----|-----|---------|--------|
| `C:\\Users\\Lab\\tf-ml3a` (JSON-esc) | `s.env` | 2 | true | ✅ |
| `C:/Users/Lab/tf-ml3a` (forward) | `s.env` | 2 | true | ✅ |
| `/c/Users/Lab/tf-ml3a` (Git Bash) | `s.env` | 0 | false | 🔴 **NÃO DETECTA** |
| ausente | `s.env` | 2 | true | ✅ (binário usa cwd nativo Windows) |

**Mecanismo confirmado para `/c/` como cwd:** `filepath.Join("/c/Users/Lab/tf-ml3a", "s.env")` = `/c/Users/Lab/tf-ml3a/s.env` → `os.Stat` falha no Windows (não é caminho válido) → não detecta.

### 2.4 Layer 2a — alvo de redirecionamento (mode=block, scope=local e global)

| Forma | rc | detected | Status |
|-------|-----|---------|--------|
| `C:\\Users\\...\\s.env` (JSON-esc backslash) | 0 | false | 🔴 **NÃO DETECTA** |
| `C:/Users/.../s.env` (forward slash) | 0 | false | 🔴 **NÃO DETECTA** |
| `/c/Users/.../s.env` (Git Bash lower) | 0 | false | 🔴 **NÃO DETECTA** |
| `/C/Users/.../s.env` (Git Bash upper) | 0 | false | 🔴 **NÃO DETECTA** |
| `s.env` (relativo) | 2 | true | ✅ |

**Resultado idêntico em scope=global.**

### 2.5 Medições adicionais (VM)

| Teste | rc | detected | Interpretação |
|-------|-----|---------|---------------|
| cwd=`C:\\...` (nativo) + arg=`/c/.../s.env` | 0 | false | Sub-caso B confirmado: `IsAbs("/c/...")=false`, `Join("C:\\...", "/c/...")` = `C:\c\...`, stat falha |
| Non-JSON + `C:\` redirect (backslash literal = JSON inválido) | 0 | false | Path não-JSON: `credNonJSONLayerTwoB` + `credCmdLineRe` para na 1ª aspa; sem detecção |
| UNC `\\\\localhost\\C$\\...\\s.env` (admin share acessível) | 2 | true | `filepath.IsAbs("\\\\...")=true` → path passado para `credScanFile` → arquivo encontrado → detecta |
| `echo key > /dev/null` | 0 | false | Isenção F2 da Layer 1: `credIsSimpleCmd=true` + `credAllTargetsAreDevNull=true` + chave no shellCmd → retorna 0 antes de qualquer scan. A regex extrai `/dev/null` completo (`:` não está em `/dev/null`); a isenção é por `credIsEphemeralTarget` em Layer 2a e pela F2 em Layer 1 — o `os.Stat` nunca é alcançado. |
| `echo key > C:\nonexistent` (Layer 1 pega primeiro) | 2 | true | Layer 1 detecta a chave no próprio shellCmd; Layer 2a não é acionada para esta linha. |

### 2.6 Prova do mecanismo de truncamento do redirect

**Método:** arquivo `C` (nome literal "C") criado em `C:\Users\Lab\tf-ml3a` contendo a chave. Payload enviado com destino de redirecionamento `C:\Users\Lab\tf-ml3a\nonexistent.env` (arquivo que não existe).

```
PROOF_C_truncation (C:\nonexistent): rc=2 detected=TRUE_PROVEN
PROOF_C_forward_truncation (C:/nonexistent): rc=2 detected=TRUE_PROVEN
```

**Interpretação:** o regex extraiu `C` (alvo fictício no cwd) → `credScanFile("C")` encontrou o arquivo → detectou a chave. Isso prova que `credRedirectRe` trunca no `:` para ambas as formas de caminho Windows com letra de unidade.

**Causa direta:** o caractere `:` está na classe de exclusão `[^ \t\r\n|&;,:]+`. Para qualquer caminho Windows com letra de unidade, o match termina antes do `:\`.

### 2.7 Probe do regex (confirmação independente)

Executado na VM com `grep -oE`. Duas variantes comparadas — antes (com `:` na classe) e depois da fix proposta (sem `:`):

| Input | Extração **antes** (atual) | Extração **depois** (fix BUG-2) | Δ |
|-------|--------------------------|--------------------------------|---|
| `echo hi > C:\Users\...\s.env` | `> C` | `> C:\Users\...\s.env` | ✅ correto após fix |
| `echo hi > C:/Users/.../s.env` | `> C` | `> C:/Users/.../s.env` | ✅ correto após fix |
| `echo hi > /c/Users/.../s.env` | `> /c/Users/.../s.env` | `> /c/Users/.../s.env` | inalterado |
| `echo hi > s.env` | `> s.env` | `> s.env` | inalterado |
| `echo hi > /dev/null` | `> /dev/null` | `> /dev/null` | inalterado — sem regressão |
| `echo hi > /tmp/out.txt:ads` | `> /tmp/out.txt` | `> /tmp/out.txt:ads` | NTFS ADS: extrai nome de stream; comportamento de `os.Stat("....:ads")` no Windows não medido — marcar como não medido; não é FP (guard não bloqueia sem achar credencial no arquivo) |

Para `/c/...`: o regex extrai o caminho completo tanto antes quanto depois. A falha de detecção em redirect `/c/...` é por BUG-1 sub-caso A (`baseCwd=""` → `if baseCwd != ""` é falso → caminho devolvido como-está → `os.Stat` falha no Windows) — não pelo regex e não por `filepath.IsAbs` (que nem é consultado no sub-caso A).

### 2.8 Modo warn e scope=global (4 linhas de controle)

| Teste | scope | rc | detected | Primeiro stderr |
|-------|-------|-----|---------|----------------|
| `cat C:\\...\\s.env` (CTRL_VAC) | local | 0 | true | `trackfw-credential-guard: warning - possible AWS access key detected in tool payload.` |
| `cat /c/.../s.env` | local | 0 | false | (vazio) |
| `echo > C:\\...\\s.env` (redirect) | local | 0 | false | (vazio) |
| `git status` (CTRL_NEG) | local | 0 | false | (vazio) |
| `cat C:\\...\\s.env` (CTRL_VAC) | global | 0 | true | `trackfw-credential-guard: warning - possible AWS access key detected in tool payload.` |
| `cat /c/.../s.env` | global | 0 | false | (vazio) |
| `echo > C:\\...\\s.env` (redirect) | global | 0 | false | (vazio) |
| `git status` (CTRL_NEG) | global | 0 | false | (vazio) |

**Conclusão:** warn e global têm padrão de detecção idêntico ao block/local para essas linhas. Em warn, rc=0 para todos; o detectou/não-detectou é lido do stderr. O FN de `/c/...` existe em warn e em block.

### 2.9 Medição macOS (no-regression para POSIX)

| Teste | rc | detected | Interpretação |
|-------|-----|---------|---------------|
| `cat <abspath>/s.env` (POSIX absoluto) | 2 | true | Layer 2b funciona corretamente no macOS para caminhos absolutos POSIX |
| `echo > <abspath>/s.env` (redirect POSIX) | 2 | true | Layer 2a funciona no macOS |
| `cat ./s.env` (relativo, controle) | 2 | true | Controle positivo ✅ |
| `cat ~/s.env` | 0 | false | Til não expandido — residual em macOS e Windows |

**Go `filepath.IsAbs` no macOS (probe Go nativo):**

| Caminho | `filepath.IsAbs` (macOS) |
|---------|------------------------|
| `/c/Users/foo/s.env` | `true` |
| `/C/Users/foo/s.env` | `true` |
| `/cygdrive/c/Users/foo` | `true` |
| `C:\\Users\\foo\\s.env` | `false` |
| `C:/Users/foo/s.env` | `false` |
| `relative/path` | `false` |
| `~/foo` | `false` |

Gate `runtime.GOOS == "windows"` obrigatório: no macOS, `C:\...` é tratado como relativo. Uma tradução não-gateada em POSIX traduziria `/c/...` para `C:\...`, que o macOS leria como relativo ao cwd — miss em vez de detecção.

### 2.10 Semântica de caminho no PowerShell (relevante para residual #6)

Medição na VM (`powershell.exe` via ssh, Windows 11 ARM64):

| Expressão PS | Resultado | Interpretação |
|-------------|-----------|---------------|
| `[System.IO.Path]::GetFullPath('/c/Users/Lab')` | `C:\c\Users\Lab` | PS resolve `/c/` como subdir de `C:\`, não como drive mapping |
| `$ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath('/c/Users/Lab')` | `C:\c\Users\Lab` | PS provider path — mesma semântica |

**Consequência para o gate GOOS-only:** se o guard traduz `/c/...` → `C:\...` para todos os tool_names, e um agente em PowerShell envia `cat /c/Users/Lab/notes.txt` (onde PS abre `C:\c\Users\...`):
- **FP:** guard scanneia `C:\Users\...` em vez de `C:\c\Users\...` — bloqueia comando inocente se `C:\Users\...` contiver credencial.
- **FN regressão:** se a credencial está em `C:\c\Users\...`, guard após fix busca em `C:\Users\...` → miss.

Gate deny-list `strings.EqualFold(toolName, "PowerShell")` resolve ambas as direções para **Claude Code** (emite `"PowerShell"` confirmado) e para **Codex** onde o caminho não seja Git Bash (Codex emite `"Bash"` mesmo em PowerShell — a tradução dispara, mas Codex não escreve caminhos `/c/...` [não medido: não concluir o contrário]). **Residual aberto para:** (a) **Codex** se o implementer assumir que `"Bash"` garante caminhos POSIX (não medido); (b) **Windsurf** (sem tool_name, não medido); — ver §5 item 6.

## 3. Formas de caminho escritas por cada CLI no Windows

| CLI | Shell/Contexto | Forma típica de caminho | Fonte |
|-----|---------------|------------------------|-------|
| Claude Code | Git Bash | `/c/Users/...` | Issue #544 (relato do usuário) |
| Claude Code | PowerShell | não medido em ML-3A | — |
| Codex | PowerShell (interno) | não medido em ML-3A — `tool_name="Bash"` confirmado (análise binária codex.exe v0.160.1 + hook logs: `docs/roadmaps/done/ROADMAP-2026-09-22...:440`); forma do caminho que escreve não medida | — |
| Windsurf | Bash/PowerShell | não medido em ML-3A | — |
| Amazon Q | Bash | não medido em ML-3A | — |

**Nota:** a forma de caminho no `tool_info.cwd` e no argumento do comando depende do shell onde o CLI corre, e pode variar por versão e configuração. ML-3A mede apenas o caso Git Bash (issue #544). As outras células requerem medição independente — não inferir.

## 4. Threat Model e Alvos de Falsificação

### 4.1 Adversário e caminho de exploit

O adversário relevante é um agente AI (como Claude Code em Git Bash) que:
1. **Passo 1:** emite `cat /c/Users/<user>/.aws/credentials` (arquivo com credencial já existente).
2. **Layer 1:** não detecta (a chave não está no payload, está no arquivo referenciado).
3. **Layer 2b — sub-caso A (sem `tool_info.cwd`):** `credResolveArg("/c/.../credentials", "")` → `baseCwd=""`, `if` é falso → devolve `/c/.../credentials` como-está → `os.Stat("/c/Users/...")` → Windows vê `C:\c\Users\...` → não existe → não detecta.
4. **Layer 2b — sub-caso B (com `tool_info.cwd` nativo):** `credResolveArg("/c/...", "C:\\Users\\...")` → `filepath.IsAbs("/c/...")=false` → `filepath.Join("C:\\...", "/c/...")` = `C:\c\...` → caminho errado → não detecta.

**Impacto:** a exfiltração da credencial passa sem alarme em modo warn e sem bloqueio em modo block.

**Realizável:** confirmado como caso real. Issue #544: "Em Git Bash, caminho é `/c/...` por padrão — é a forma que um agente escreve naturalmente."

**Segunda superfície (redirect):** mesmo cenário usando redirecionamento como proxy de varredura:
`cat > /c/Users/.../s.env` → Layer 2a extrai `/c/.../s.env` → `credResolveArg` falha (mesmo mecanismo) → não detecta.

**Terceira superfície (redirect com caminho nativo):** `echo secret > C:\Users\...` → Layer 2a extrai `C` (truncado pelo `:`) → `credScanFile("C")` → procura arquivo chamado `C` no cwd → não encontra → não detecta. Mas SE o cwd tiver um arquivo chamado `C` (caso improvável mas não impossível), o guard detecta o arquivo errado.

### 4.2 Alvos de falsificação — direção FN (guard deve bloquear mas não bloqueia)

| Superfície | Onde entra | Qual gate deveria pegar | Vetor FN medido |
|-----------|-----------|------------------------|----------------|
| Layer 2b arg, `/c/...` (sem cwd) | `credResolveArg:513 return clean` — sub-caso A; `os.Stat("/c/...")` falha | `credScanFile` retorna match | `cat /c/...` → rc=0 (warn e block) |
| Layer 2b arg, `/c/...` (cwd nativo) | `credResolveArg:510` — `filepath.IsAbs=false` → join errado; sub-caso B | Idem | cwd nativo + `/c/` arg → rc=0 |
| Layer 2b arg, `/C/...` | Idem que `/c/` | Idem | `cat /C/...` → rc=0 |
| Layer 2b arg, `/cygdrive/c/...` | Idem — sub-casos A e B | Idem | `cat /cygdrive/c/...` → rc=0 |
| Layer 2b arg, `~/...` | `credResolveArg:513` — `~/s.env` como-está → `os.Stat("~/s.env")` falha (`~` não expandido pelo Go) | Idem | `cat ~/...` → rc=0 |
| Layer 2b cwd, `/c/...` | `credExtractToolInfoCwd` devolve `/c/...` → `credResolveArg:511` join(`/c/cwd`, `rel.env`) = `/c/cwd/rel.env` → stat falha | Join correto com cwd | cwd=/c/..., arg=relative → rc=0 |
| Layer 2a redirect, `C:\...` | `credRedirectRe:30` — `:` exclui → regex extrai `C` (BUG-2) | `credScanFile("C")` encontra arquivo `C` no cwd | `echo > C:\...` → rc=0 |
| Layer 2a redirect, `C:/...` | Idem BUG-2 | Idem | `echo > C:/...` → rc=0 |
| Layer 2a redirect, `/c/...` | Regex extrai caminho completo; BUG-1 sub-caso A falha em stat | `credScanFile` | `echo > /c/...` → rc=0 |
| Glob, `/c/.../s.*` | `credGlobToken:304` — `filepath.Glob("/c/...")` = nil; literal devolvido → BUG-1 | Token expandido + scan | glob `/c/.../s.*` → rc=0 |

### 4.3 Alvos de falsificação — riscos na implementação do ML-3B

A fix proposta pode introduzir dois tipos de regressão se implementada incorretamente.

**Risco FN — fix sem gate em `runtime.GOOS == "windows"` (tradução aplicada também no POSIX):**

Em Linux/macOS, `/c/Users/...` é um caminho POSIX absoluto legítimo (`filepath.IsAbs` = `true`). Se `credNormalizeWindowsPath` rodar no POSIX, ela traduz `/c/...` para `C:\...`. No macOS, `filepath.IsAbs("C:\\Users\\...")=false` (medido: §2.9, Go probe) → tratado como relativo → join com cwd → caminho errado → miss. Prova via §2.9: `cat <abspath>/s.env` (POSIX absoluto) → rc=2 detected=true confirma que o comportamento correto no POSIX depende de não aplicar a tradução.

**Risco FN — normalização aplicada antes da strip de aspas/`},`:**

Se `credNormalizeWindowsPath` for chamada antes do `strings.TrimRight(clean, "},")`, um token como `"/c/Users/..."` pode conter aspa residual → a regex de forma Git Bash não casa → path não é traduzido. A chamada deve ser APÓS o TrimRight e o check de empty (após linha 508, antes da linha 510 em credential.go).

**Risco de regressão — `credRedirectRe` sem `:` e NTFS ADS (`file.txt:stream`):**

Após remover `:`, `echo > file.txt:stream` extrai `file.txt:stream`. Comportamento de `os.Stat("file.txt:stream")` no Windows: não medido. Efeito hoje (com `:`): extrai `file.txt` → se arquivo existe, é scanneado. Depois: extrai `file.txt:stream` → stat diferente (potencial miss-vs-miss). Não é FP (guard só bloqueia se encontrar credencial).

**Risco FP/FN por `tool_name` — PowerShell nativo com path `/c/...`:**

O guard extrai `tool_name` (credential.go:436) mas atualmente só para `fs_write`. Medido na VM: `[System.IO.Path]::GetFullPath('/c/Users/Lab')` = `C:\c\Users\Lab`. Com GOOS-only gate:
- **FP:** PS envia `cat /c/Users/Lab/notes.txt` → guard traduz → `C:\Users\Lab\notes.txt` → arquivo com chave → bloqueia (PS realmente abriria `C:\c\Users\Lab\notes.txt`).
- **FN regressão:** chave em `C:\c\Users\...`. Hoje `os.Stat("/c/Users/...")` → `C:\c\Users\...` (Windows) → pode detectar. Após fix: guard busca em `C:\Users\...` → miss.

**Decisão para spec do ML-3B:** gate `tool_name != "PowerShell"` (deny-list: traduzir para todo `tool_name` exceto `"PowerShell"`). Isso cobre Git Bash (`"Bash"`), Amazon Q (`"execute_bash"`) e Windsurf (tool_name ausente — por omissão, cai no deny-list e é traduzido). Medição §2.10 confirma ambas as direções do residual se não implementado.

**Prova POSIX (`filepath.IsAbs`) — probe Go nativo no macOS:**
```
filepath.IsAbs("/c/Users/foo/s.env") = true   ← POSIX absoluto
filepath.IsAbs("C:\\Users\\foo\\s.env") = false  ← Windows relativo no POSIX
```
Medição §2.9: `cat <abspath>/s.env` → rc=2 detected=true ✅; `echo > <abspath>/s.env` → rc=2 detected=true ✅.
`~/s.env` → rc=0 detected=false (til não expandido — residual em macOS e Windows).

### 4.4 Relação com a ADR-2026-09-04

A ADR-2026-09-04 (D2) determina: `filepath.IsAbs` continua para caminho que o SO abre. A decisão D1 (predicado de ancoragem) aplica-se a caminhos em arquivos de configuração interpretados pelo bash, não a caminhos abertos por syscall.

Aqui o contexto é diferente: o caminho em `tool_input.command` é escrito pelo CLI do agente (Git Bash) e usa semântica POSIX. O guard precisa abrir o arquivo no Windows. O gap é: o CLI escreve em gramática POSIX, o guard abre em gramática Windows. A tradução `/X/...` → `X:\...` é o elo faltante, e ela só faz sentido quando `runtime.GOOS == "windows"` (sem isso, a tradução quebraria o POSIX).

## 5. Residual Declarado

1. **Caminhos MSYS que não são letras de unidade** (`/tmp`, `/home`, `/usr`, `/usr/local`): a regra de tradução de drive (`/X/...` → `X:\...`) não cobre esses prefixos. Um comando `cat /tmp/secret.env` no Git Bash não é detectável pela Layer 2b. Resíduo pré-existente — não é introduzido por esta Wave.

2. **UNC** (`\\server\share\...`): medido e **detectado** quando o compartilhamento é acessível (VM: admin share C$ habilitado; `cat \\\\localhost\\C$\\...\\s.env` → rc=2 detected=true). `filepath.IsAbs("\\\\...")=true` → path passado para `credScanFile` → arquivo encontrado. Comportamento correto; sem defeito estrutural.

3. **`~/...` (til):** não expandido pelo guard — nem no Windows nem no macOS (medido em ambos: rc=0 detected=false). O shell expandiria `~/...` antes de enviar, então o payload real raramente conterá `~`. Se contiver, não detecta. Resíduo pré-existente nos dois sistemas operacionais.

4. **Caminhos com aspas e backslash literal** (`"C:\..."` no payload): torna o JSON inválido (`\U` não é escape JSON). O path não-JSON extrai via `credCmdLineRe` que para na aspa. Não é o caso de uso real (CLIs enviam JSON válido com `C:\\...`). Resíduo de edge case.

5. **Conteúdo do arquivo criado vs. conteúdo do arquivo lido:** a Layer 2a varre arquivos já existentes com credencial. Um arquivo recém-criado (pelo passo 1 do exploit) só seria detectado se a Layer 2a fosse acionada em seguida. A janela entre escrita e leitura é uma janela de exposição inerente ao design.

6. **PowerShell nativo com caminho `/c/...` no payload (duas direções; parcialmente fechado):** medido na VM (§2.10): `[System.IO.Path]::GetFullPath('/c/Users/Lab')` = `C:\c\Users\Lab`. Com gate GOOS-only: (a) **FP** — PS envia `cat /c/Users/Lab/notes.txt`, guard traduz para `C:\Users\Lab\notes.txt`, scanneia, encontra chave → bloqueia comando inocente; (b) **FN regressão** — chave em `C:\c\Users\...`, guard pós-fix busca em `C:\Users\...` → miss. O gate deny-list `strings.EqualFold(toolName, "PowerShell")` fecha o residual para **Claude Code** (emite `"PowerShell"` confirmado: `docs/portabilidade/...line 646`). **Permanece aberto para:** (a) **Codex** (emite `"Bash"` mesmo em PowerShell — ROADMAP-2026-09-22...:440; forma dos caminhos que escreve não medida em ML-3A: se Codex escreve `/c/...` em PS, o deny-list traduz incorretamente); (b) **Windsurf** (sem tool_name, §3 não medido — cai no deny-list por omissão). Se não implementado no ML-3B (gate deny-list), ambas as direções persistem para todos os CLIs.

## 6. Veredito

### Bugs confirmados e medidos

**BUG-1 (CRÍTICO, Layer 2b + Layer 2a) — dois sub-casos:**
- **Sub-caso A (baseCwd=""):** `credResolveArg` devolve `/c/Users/...` como-está (linha 513). `os.Stat("/c/Users/...")` no Windows interpreta `/c/` como `C:\c\` → arquivo não existe. `filepath.IsAbs` não é consultado neste sub-caso.
- **Sub-caso B (baseCwd="C:\\..."):** `filepath.IsAbs("/c/...")=false` → `filepath.Join("C:\\...", "/c/...")` = `C:\c\...` → caminho errado → stat falha.
Ambos medidos: rc=0 detected=false.

**BUG-2 (CRÍTICO, Layer 2a):** `credRedirectRe` exclui `:` da classe de caracteres. Para `echo > C:\path` ou `echo > C:/path`, o regex extrai apenas `C` (a letra de unidade), não o caminho completo. O arquivo alvo não é encontrado.

### Especificação do ML-3B

**O que traduzir:** caminhos na forma Git Bash (`/[a-zA-Z]/...` onde o restante não começa com outro `/`), formato Cygdrive (`/cygdrive/[a-zA-Z]/...`). Não traduzir: `/tmp`, `/home`, `/usr`, `/opt` e outros prefixos MSYS sem letra de unidade.

**Onde traduzir:** criar função `credNormalizeWindowsPath(path, goos, toolName string) string` que:
- SE `goos != "windows"`: retorna `path` sem alteração.
- SE `toolName == "PowerShell"`: retorna `path` sem alteração (deny-list; PS usa API Windows nativa — `/c/...` → `C:\c\...`).
- SE `path` corresponde a `/[a-zA-Z]/...` (letra de unidade Git Bash): traduz para `X:/<rest>` (forward slash — evita `filepath.FromSlash` que é OS-dependente nos testes; `C:/...` funciona no guard: §2.2 `C:/...` → rc=2 detected=true).
- SE `path` corresponde a `/cygdrive/[a-zA-Z]/...`: traduz para `X:/<rest>` (forward slash, idem).
- Caso contrário: retorna `path` sem alteração.

O argumento `goos` é `runtime.GOOS` nos callsites de produção; injetado como `"windows"` ou `"linux"` nos testes. Isso elimina a necessidade de cross-compilar para exercer os dois ramos. A função deve usar apenas `strings` e `regexp` — nunca funções de `path/filepath` (`Clean`, `ToSlash`, `VolumeName` etc.) que mudam comportamento por OS e quebrariam as expectativas de forward-slash em testes de CI não-Windows.

**Onde chamar:**
1. Em `credResolveArg` (credential.go:509): **após** `clean = strings.TrimRight(clean, "},")` e **após** o `if clean == "" { return arg }` (linha 507–508), **antes** da linha `if baseCwd != "" && !filepath.IsAbs(clean)` (linha 510). Isso garante que `clean` já tem aspas e `,}` removidos antes da tradução, E cobre tanto o sub-caso A (`baseCwd=""` — path devolvido como-está) quanto o sub-caso B (`baseCwd≠""` — IsAbs após tradução = true → não faz join).
2. Na função `credGlobToken` (linha 298), antes de chamar `filepath.Glob(pattern)`, para normalizar o padrão.
3. Em `credExtractToolInfoCwd` (linha 472), para normalizar o cwd extraído do payload antes de devolvê-lo (assim todos os chamadores de `credSecondLayer` que usam shellCwd recebem cwd em formato Windows).

**Correção do regex (BUG-2):** remover `:` da classe de exclusão em `credRedirectRe`:

```go
// Antes:
credRedirectRe = regexp.MustCompile(`[0-9]?>>?[ \t]*[^ \t\r\n|&;,:]+`)
// Depois:
credRedirectRe = regexp.MustCompile(`[0-9]?>>?[ \t]*[^ \t\r\n|&;,]+`)
```

**Justificativa para remoção de `:` no regex:** `:` em targets de redirecionamento é raro em shell válido (não há semântica de `:` em destino de redirect). O risco de match excessivo é baixo: `credScanFile` falha graciosamente para caminhos inválidos. A prova de não-regressão: o controle negativo `git status` (rc=0 detected=false) e o controle de payload inocente devem permanecer.

**Gate `goos`:** a tradução DEVE ser gated em `goos == "windows"` (injetando `runtime.GOOS` em produção). O probe Go nativo confirmou que `/c/...` é `IsAbs=true` em macOS; Linux segue a mesma implementação `path/filepath` unix — sem diferença de comportamento. Uma tradução não-gateada quebraria o comportamento correto no POSIX (no macOS, `C:\...` é falso absoluto → relativo → miss).

**Gate adicional por `tool_name` (para eliminar residual #6 — deny-list):** gate `strings.EqualFold(toolName, "PowerShell")` — skip tradução para PowerShell, que usa Windows API nativa (`GetFullPath`). String `"PowerShell"` confirmada em payload real de Claude Code 2.1.292+ (`docs/portabilidade/2026-10-04-trackfw-no-path-dos-shells-do-windows-por-canal.md:646`). `EqualFold` em vez de `==`: nenhum shell se chamará `"powershell"` em lowercase para fins de MSYS2, mas a comparação insensível a case é mais robusta contra variações futuras de capitalização sem custo.

**Cadeia de assinatura obrigatória para propagar `toolName` (assinaturas atuais lidas de credential.go):**

| Função | Assinatura atual | Assinatura pós-ML-3B |
|--------|-----------------|---------------------|
| `credExtractCmdAndCwd` | `(data []byte) (shellCmd, cwd string, isJSON bool)` | `(data []byte) (shellCmd, cwd, toolName string, isJSON bool)` — extrair `tool_name` de `root["tool_name"]` (bloco já existe em cred.go:436) |
| `credExtractToolInfoCwd` | `(root map[string]json.RawMessage) string` | `(root map[string]json.RawMessage, toolName string) string` — normalizar cwd com `credNormalizeWindowsPath(cwd, goos, toolName)` |
| `credSecondLayer` | `(shellCmd, shellCwd, contextStr string, redirectMatches []string) string` | `(shellCmd, shellCwd, contextStr, toolName string, redirectMatches []string) string` — propagar toolName para credResolveArg |
| `credResolveArg` | `(arg, baseCwd string) string` | `(arg, baseCwd, toolName string) string` — chamar `credNormalizeWindowsPath(clean, goos, toolName)` no ponto especificado |
| `credGlobToken` | `(token string) []string` | `(token, toolName string) []string` — normalizar `token` antes de `filepath.Glob` |
| `credDeepScan` | `(data []byte, shellCwd string) string` | `(data []byte, shellCwd string) string` — extrai `tool_name` de `m["tool_name"]` (bloco já existe em cred.go:721) e passa para `credSecondLayer` |
| `credNonJSONLayerTwoB` | — | passa `toolName=""` para `credSecondLayer` (path não-JSON não tem tool_name parseável; `""` não é `"PowerShell"` → deny-list traduz → correto) |

Todos os chamadores de `credSecondLayer` (`RunCredential`, `RunCredentialGlobal`, `credDeepScan`, `credNonJSONLayerTwoB`) recebem `toolName` de suas respectivas fontes de extração.

Se `toolName` for passado como `""` em qualquer ponto, a deny-list (com gate `toolName == "PowerShell"`) nunca dispara → tradução sempre aplicada → residual #6 volta silenciosamente. O par de integração abaixo é o único que pega este furo.

**Tabela de testes obrigatórios para ML-3B:**

| Forma | Escopo | Sítio | Resultado esperado | Frase de reconciliação |
|-------|--------|-------|-------------------|----------------------|
| `/c/Users/.../s.env` (sem cwd no payload) | Windows local | Layer 2b arg, sub-caso A | detected=true | Tradução `/c/...`→`C:\...` antes do retorno; sub-caso A (baseCwd="") coberto |
| `/c/Users/.../s.env` (cwd nativo `C:\\...`) | Windows local | Layer 2b arg, sub-caso B | detected=true | Tradução → IsAbs=true → não faz join errado |
| `/C/Users/.../s.env` (upper, sem cwd) | Windows local | Layer 2b arg | detected=true | Case-insensitive na regex de tradução |
| `/cygdrive/c/.../s.env` | Windows local | Layer 2b arg | detected=true | Tradução `/cygdrive/c/`→`C:\` |
| `C:\...` (JSON-esc) | Windows local | Layer 2b arg | detected=true | Regressão: forma existente continua |
| `C:/...` | Windows local | Layer 2b arg | detected=true | Regressão: forma existente continua |
| `/c/...` como cwd + `s.env` relativo | Windows local | Layer 2b via cwd | detected=true | credExtractToolInfoCwd normaliza cwd |
| `echo > C:\...` (JSON-esc) | Windows local | Layer 2a redirect | detected=true | Regex sem `:` extrai caminho completo |
| `echo > C:/...` | Windows local | Layer 2a redirect | detected=true | Idem |
| `echo > /c/.../s.env` | Windows local | Layer 2a redirect | detected=true | Regex extrai caminho completo + tradução resolve |
| `echo > s.env` (relativo) | Windows local | Layer 2a redirect | detected=true | Regressão: controle existente |
| `cat ./s.env` (relativo, POSIX) | macOS local | Layer 2b arg | detected=true | GOOS gate: comportamento inalterado no POSIX |
| `echo > /tmp/s.env` (POSIX real) | macOS local | Layer 2a redirect | detected=true | Regressão POSIX: comportamento inalterado |

**Falsificação obrigatória no ML-3B:**

`credNormalizeWindowsPath` deve aceitar `goos string` e `toolName string` injetáveis. Testes de unidade sobre a função discriminam diretamente — os testes de integração abaixo são complementares.

**Unit tests obrigatórios para `credNormalizeWindowsPath(path, goos, toolName)`:**

| Entrada | goos | toolName | Saída esperada | Sabotagem que o teste pega |
|---------|------|----------|----------------|---------------------------|
| `/c/Users/x/s.env` | `"windows"` | `"Bash"` | `C:/Users/x/s.env` | normalização desativada no Windows |
| `/C/Users/x/s.env` | `"windows"` | `"Bash"` | `C:/Users/x/s.env` | case-insensitivity |
| `/cygdrive/c/Users/x` | `"windows"` | `"Bash"` | `C:/Users/x` | cygdrive não coberto |
| `/c/Users/x/s.env` | `"linux"` | `"Bash"` | `/c/Users/x/s.env` | gate GOOS removido (FN em POSIX) |
| `/tmp/s.env` | `"windows"` | `"Bash"` | `/tmp/s.env` | regex over-broad (FN de caminhos MSYS sem drive) |
| `/home/user/s.env` | `"windows"` | `"Bash"` | `/home/user/s.env` | idem |
| `C:\Users\x\s.env` | `"windows"` | `"Bash"` | `C:\Users\x\s.env` | regressão: forma nativa não alterada |
| `relative/s.env` | `"windows"` | `"Bash"` | `relative/s.env` | regressão: relativo não alterado |
| `/c/Users/x/s.env` | `"windows"` | `"PowerShell"` | `/c/Users/x/s.env` | deny-list PS (EqualFold): tradução não aplicada |
| `/c/Users/x/s.env` | `"windows"` | `"powershell"` | `/c/Users/x/s.env` | EqualFold: variação lowercase também excluída |
| `/c/Users/x/s.env` | `"windows"` | `"execute_bash"` | `C:/Users/x/s.env` | Amazon Q (deny-list só exclui PS) |
| `/c/Users/x/s.env` | `"windows"` | `""` | `C:/Users/x/s.env` | Windsurf (tool_name ausente cai no deny-list → traduzido) |

**Test de integração complementar (test table em §6 + rows adicionais):**

Adicionar ao test table no ML-3B (além das linhas já listadas):

| Forma | Escopo | Sítio | Resultado esperado | Frase de reconciliação |
|-------|--------|-------|-------------------|----------------------|
| `echo <key> > /dev/null` | Windows local | F2 exemption | rc=0 detected=false | F2 (`credAllTargetsAreDevNull`) sobrevive à mudança do regex |
| `echo <key> > C:\x` | Windows local | Layer 1 + regex fix | rc=2 detected=true | Layer 1 detecta chave no payload; regex fix não interfere |
| `cat /c/.../s.*` (glob) | Windows local | credGlobToken | detected=true | Glob normalizado + expandido corretamente |

**Par de integração obrigatório para `tool_name` gate (o único que prova a fiação):**

| Forma | tool_name no payload | Escopo | Sítio | Resultado esperado | Frase de reconciliação |
|-------|---------------------|--------|-------|-------------------|----------------------|
| `cat /c/Users/.../s.env` | `"Bash"` | Windows local | Layer 2b arg | detected=true | Gate deny-list não exclui Bash → traduz `/c/...` → `C:/...` → scan acha credencial |
| `cat /c/Users/.../s.env` | `"PowerShell"` | Windows local | Layer 2b arg | detected=false | Gate deny-list exclui PS → sem tradução → `os.Stat("/c/...")` falha → miss (comportamento esperado; PS abriria `C:\c\...`) |

Nota: o arquivo deve existir em `C:/Users/.../s.env` (caminho traduzido) com credencial. Para a linha PowerShell, `detected=false` é o resultado correto — o guard não tenta abrir `C:\c\Users\...` (onde PS abriria), mas também não bloqueia inocentemente `C:\Users\...`.

**Nota sobre `credNonJSONLayerTwoB`:** essa função passa `shellCwd=""` para todos os casos não-JSON. A normalização do cwd não a afeta diretamente, mas a normalização do argumento em `credResolveArg` a beneficia também. `credNonJSONLayerTwoB` chama `credSecondLayer` com `toolName=""` (path não-JSON não tem `tool_name` parseável) — deny-list com `toolName=""` não é `"PowerShell"` → traduz → comportamento correto para o caso não-JSON.
