---
title: "Wave 0 -- Threat Model do guard em Go (ADR-2026-10-04)"
date: 2026-10-04
author: hades-tf
status: concluido
roadmap: ROADMAP-2026-09-22-os-hooks-de-guard-nao-executam-no-windows-na-maioria-dos-clis-de-agente-e-o-validate-reporta-instalado.md
ml: ML-0A
---

# Wave 0 -- Threat Model do guard em Go

> 2026-10-04 | Hades (Security) | ML-0A do ROADMAP-2026-09-22
>
> Consome: ML-0B (`docs/portabilidade/2026-10-04-trackfw-no-path-dos-shells-do-windows-por-canal.md`)
> e ML-0C (`docs/portabilidade/2026-10-04-remedicao-do-schema-de-hook-dos-clis-de-agente.md`).
>
> Marcadores: **medido** = comando executado neste contexto com saida colada;
> **documentado** = fonte citada; **[inferido]** = derivado de premissa verificavel, nao medido ao vivo.

---

## Secao 1 -- Completude da enumeracao

### Diretorios ausentes (saida verbatim)

```
$ ls .kiro/ .windsurf/ .amazonq/ .cursor/ .github/hooks/ plugins/ 2>&1
".kiro/": No such file or directory (os error 2)
".windsurf/": No such file or directory (os error 2)
".amazonq/": No such file or directory (os error 2)
".cursor/": No such file or directory (os error 2)
".github/hooks/": No such file or directory (os error 2)
"plugins/": No such file or directory (os error 2)
```

**medido** 2026-10-04.

### Sitios encontrados por papel

**Geradores:**

| Arquivo | Ocorrencias relevantes |
|---|---|
| `internal/generators/scaffold.go` | `gitBranchGuardScript` const L1713; `credentialGuardScript` const L1564; `globalCredentialGuardScript` const L1571; escrita em L1250, L1601, L1654 |
| `internal/generators/scaffold_doctor.go` | L281-282 regenera ambos os scripts corrompidos |
| `internal/generators/agentfiles.go` | `claudeGitGuardCmd` L449, `codexGitGuardCmd` L450, `geminiGitGuardCmd` L451, `windsurfGitGuardCmd` L1353, `amazonQGitGuardCmd` L1618; 8 funcoes `Inject*Hooks` |
| `internal/generators/update.go` | global `~/.trackfw/scripts/trackfw-*.sh` L735-885 e L969-1540 |

**Validator:**

| Arquivo | Papel |
|---|---|
| `internal/validator/validator_credential_guard.go` | `credentialGuardScriptMarker` L17; `gitBranchGuardScriptMarker` L22; `validateGuardHookResolvable()`; `collectCommandsWithMarker` usa `strings.Contains` (substring match, nao exact) |
| `internal/validator/validator_credential_guard_integrity_reference.go` | copia de referencia (escopo projeto) |
| `internal/validator/validator_credential_guard_global_reference.go` | copia de referencia (global) |
| `internal/validator/validator_git_branch_guard_reference.go` | `gitBranchGuardScriptReference` |
| `internal/validator/validator_git_branch_guard.go` | integridade project+global |

**CI/Qualidade:**

`scripts/check-git-branch-guard-hook-schema.sh` executa `bash scripts/trackfw-git-branch-guard.sh` diretamente (L297, L323, L361, L612, L634-639). Apos ML-2A converter o script em `exec trackfw guard ...`, o check invoca o wrapper sem o binario e quebra. Deve ser atualizado no ML-2A.

**Configuracoes de hook do repositorio (pre-migracao):**

`.claude/settings.json` (7 ocorrencias `$CLAUDE_PROJECT_DIR/scripts/trackfw-*.sh`), `.codex/hooks.json` (5), `.gemini/settings.json` (7). Precisam de migracao no ML-2A.

**npm e pypi:**

```
$ grep -r "trackfw-git-branch-guard\|gitBranchGuardScript" npm/ pypi/ 2>&1
(vazio, rc=1)
```

**medido**.

**Tamanhos:**

```
$ wc -l scripts/trackfw-git-branch-guard.sh scripts/trackfw-credential-guard.sh
     756 scripts/trackfw-git-branch-guard.sh
     152 scripts/trackfw-credential-guard.sh
     908 total
```

**medido**.

### Referencias a "trackfw" nos scripts -- caminho de decisao

**git-branch guard (L1-756):**

```
$ grep -o trackfw scripts/trackfw-git-branch-guard.sh | wc -l
44
$ grep -c trackfw scripts/trackfw-git-branch-guard.sh
29
```

**medido** -- `grep -o` (ocorrencias) = 44: reproduz exatamente o numero da ADR-2026-10-04. `grep -c` (linhas) = 29. A ADR conta ocorrencias, nao linhas. Com filtro de comentarios `^[^#]*trackfw`: 19 linhas. **Conclusao:** os 44 da ADR sao corretos como contagem de ocorrencias; a afirmacao "guards ja chamam o trackfw" e a inferencia errada -- todas as 44 ocorrencias sao REASON strings, referencias a `trackfw.yaml`, strings AWK. Nenhuma invoca o binario.

Verificacao uppercase -- `TRACKFW` sem prefixo `_TRACKFW_`:

```
$ grep -n "TRACKFW" scripts/trackfw-git-branch-guard.sh | grep -v "_TRACKFW_"
143:#       ... "$TRACKFW_GIT_COMMAND" NAO isenta ...
281:  CMD_RAW="${TRACKFW_GIT_COMMAND:-}"
```

**medido**. `TRACKFW_GIT_COMMAND` e variavel de ambiente de fallback de terceiro nivel (L281-282): usada apenas quando stdin e argv estao ambos vazios.

**credential guard (L1-152):**

```
$ grep -n trackfw scripts/trackfw-credential-guard.sh | wc -l
8
```

**medido** (8). Todas sao: comentario, `[ -f "trackfw.yaml" ]`, grep de `trackfw.yaml`, strings de mensagem, escrita de arquivo de saida. Nenhuma invoca o binario.

Ambos os guards NAO invocam `trackfw` no caminho de decisao. A dependencia de PATH e nova, contradizendo a afirmacao da ADR-2026-10-04 ("os guards ja chamam o trackfw (44 e 8 ocorrencias)").

### Canais de entrada e comportamento do loop de stdin

**Loop de leitura do git-branch guard (L86-101):**

```bash
_TRACKFW_STDIN=""
_TRACKFW_STDIN_TRUNCATED=0
while :; do
  _TRACKFW_STDIN_CHUNK=""
  _TRACKFW_STDIN_RC=0
  IFS= read -r -t 2 -d '' _TRACKFW_STDIN_CHUNK || _TRACKFW_STDIN_RC=$?
  _TRACKFW_STDIN="${_TRACKFW_STDIN}${_TRACKFW_STDIN_CHUNK}"
  if [ "$_TRACKFW_STDIN_RC" -le 128 ]; then  # EOF ou NUL -> break
    break
  fi
  if [ -z "$_TRACKFW_STDIN_CHUNK" ]; then    # timeout com chunk vazio -> TRUNCATED
    _TRACKFW_STDIN_TRUNCATED=1
    break
  fi
  # timeout com chunk nao-vazio -> acumula e itera
done
```

**medido** -- leitura L86-101. Semantica: acumula chunks de 2s; para em EOF/NUL (correto) ou em timeout-com-chunk-vazio (TRUNCATED -> nega se sem argv).

**Medicoes de comportamento dos canais (medido 2026-10-04):**

```
$ time (sleep 3) | bash scripts/trackfw-git-branch-guard.sh; echo idle:$?
[deny: stdin truncado apos 2s sem dados] -- 2.016s total
idle:2

$ bash scripts/trackfw-git-branch-guard.sh 'git push origin main' </dev/null; echo argv:$?
[deny JSON]
argv:2

$ TRACKFW_GIT_COMMAND='git push origin main' bash scripts/trackfw-git-branch-guard.sh </dev/null; echo env:$?
[deny JSON]
env:2

$ printf '{"tool_input":{"command":"git push origin main"}}' \
    | bash scripts/trackfw-git-branch-guard.sh exit '$LASTEXITCODE'; echo argv_overrides_stdin:$?
(sem saida JSON; exit 0 silencioso)
argv_overrides_stdin:0

$ bash scripts/trackfw-git-branch-guard.sh git push --force origin main </dev/null; echo argv_split:$?
[deny JSON]
argv_split:2

$ time (printf '{"tool_input":{"command":"git pu'; sleep 3) | bash scripts/trackfw-git-branch-guard.sh; echo partial:$?
[deny: extrator JSON "unterminated_string"] -- 3.034s total
partial:2

$ time (printf '{"tool_input":{"command":"echo ok"}}'; sleep 3) | bash scripts/trackfw-git-branch-guard.sh; echo complete_noeof:$?
(nenhum JSON de saida; exit 0) -- 3.054s total
complete_noeof:0
```

**medido** -- todos em 2026-10-04.

**Interpretacao das medicoes:**

- `idle:2` -- stdin ocioso por 2s (nenhum byte), chunk vazio -> TRUNCATED -> nega. Timeout de 2s e de OCIOSIDADE, nao de leitura total.
- `argv:2`, `env:2` -- canais de fallback funcionam.
- `argv_overrides_stdin:0` -- **FN CONFIRMADO NO .SH:** se argv esta presente (`$# > 0`), o script usa `CMD_RAW = "$*"` (L157). Com argv "exit" "$LASTEXITCODE" como tokens, `CMD_RAW` = "exit $LASTEXITCODE", que nao casa com nenhum padrao git bloqueado -> permite. Stdin de bloqueio ignorado. O leak de sufixo cmd.exe via argv e fail-open demonstrado na implementacao de referencia.
- `argv_split:2` -- argv com `--force` correto: o script junta `$*` como string e casa "git push --force origin main".
- `partial:2` -- payload incompleto + EOF em 3s -> nega. O loop acumula o chunk parcial (< 2s de leitura), entao na segunda iteracao o EOF chega (pipe fecha), RC <= 128, break; TRUNCATED=0. O JSON incompleto e parseado -> erro -> nega.
- `complete_noeof:0` -- payload enviado em t=0, pipe fechado em t=3s. Primeira iteracao de `read -t 2 -d ''`: le payload (dados imediatos), expira em t=2s com chunk=payload, RC>128 -> acumula, itera. Segunda iteracao comeca em ~t=2s; EOF chega em t=3s (1s de espera, dentro da janela de 2s), RC<=128 -> break, TRUNCATED=0 -> permite.
- **late_eof -- FP CONFIRMADO (medido 2026-10-04, implicacao bilateral):**

```
$ time (printf '{"tool_input":{"command":"echo ok"}}'; sleep 6) | bash scripts/trackfw-git-branch-guard.sh; echo late_eof_benign:$?
[deny: stdin truncado apos 2s sem dados] -- 4.024s total
late_eof_benign:2

$ time (printf '{"tool_input":{"command":"git push origin main"}}'; sleep 6) | bash scripts/trackfw-git-branch-guard.sh; echo late_eof_deny:$?
[deny: stdin truncado apos 2s sem dados] -- 4.032s total
late_eof_deny:2
```

Segunda iteracao comeca em ~t=2s; EOF chega em t=6s (4s de espera, > 2s da janela), janela expira com chunk vazio -> TRUNCATED=1 -> nega. **Ambos casos (benigno e bloqueado) negam identicamente.** Latencia: ~4s ate a negacao. **Semantica correta:** guard nega quando uma janela de 2s retorna VAZIA; nao e "2s apos o ultimo byte" -- e se o EOF chega dentro de qualquer janela de 2s em andamento, permite. **Implicacao para Go:** um `io.ReadAll` simples ou um loop com deadline resetado a cada byte implementaria o comportamento incorretamente. O gate de paridade ML-1A deve usar o par medido: `(payload; sleep 3) | guard` -> allow; `(payload; sleep 6) | guard` -> deny em ~4s. bash 5.3.20 (medido).

**Credential guard -- stdin sem timeout:**

```
$ date +%s; (sleep 3) | bash scripts/trackfw-credential-guard.sh; echo cred_idle:$?; date +%s
1791149178
cred_idle:0
1791149181
```

**medido** -- L5: `INPUT=$(cat)` sem timeout; aguarda EOF. Exit 0 (permitido) apos 3s porque `[ -f "trackfw.yaml" ]` passa neste repo. Nenhum arquivo `.trackfw-credential-guard.json` criado (verificado: `ls -la docs/roadmaps/.trackfw-credential-guard.json 2>&1` -> rc=1).

### Gap: `globalCredentialGuardScript` sem porta explicita

`scaffold.go:1571` -- variante global usa `credentialGuardGlobalTail`. ML-1B porta apenas escopo de projeto. Decisao necessaria antes do ML-1B.

### `corrupt_literal`

Em `internal/roadmapdoc/testdata/corpus/` (excluido pelo filtro) e em comentario `internal/validator/validator.go:2241`. Nao e sitio de emissao do guard.

### Veredito

Lista fechada para codigo de produto. Adicoes vs. escopo declarado: (1) `scripts/check-git-branch-guard-hook-schema.sh`, (2) `docs/cli-parity.md`, (3) `globalCredentialGuardScript`, (4) 3 configs de hook do repositorio, (5) canais de entrada (loop de stdin, argv, env) como superficie.

---

## Secao 2 -- Threat Model

O adversario e o implementador apressado e o arquiteto otimista.

### (a) Shim npm + PowerShell Restricted

**Contexto medido (ML-0B):** sob `ExecutionPolicy Restricted`, PS resolve `trackfw.ps1` -> `PSSecurityException`, exit 1.

**CLIs afetados:**

| CLI | Shell | npm+Restricted | Consequencia |
|---|---|---|---|
| Codex | PowerShell | afetado | exit 1 -> fail-open |
| Windsurf | PowerShell | afetado | exit 1 -> fail-open |
| Claude Code (PS fallback) | PowerShell | afetado | exit 1 -> fail-open |
| Cursor | PS + `-ExecutionPolicy Bypass` [ML-0C] | NAO afetado | -- |
| Copilot | PowerShell | afetado | exit 1 -> deny-all |
| Kiro | cmd.exe | NAO afetado | -- |
| Amazon Q | cmd.exe | NAO afetado | -- |
| Claude Code (Git Bash) | bash | NAO afetado | -- |

3 CLIs fail-open; 1 deny-all; 4 nao afetados.

**RemoteSigned:** provavelmente NAO bloqueia shim npm (sem MoTW). ML-0D verifica: `Get-Item "$(npm prefix -g)\bin\trackfw.ps1" -Stream Zone.Identifier`.

**Candidatos:** (1) validate orienta RemoteSigned se sem MoTW; (2) npm entrega exe nativo (fora de escopo); (3) declarar unsupported. **Decisao KG.**

**Severidade: alta.**

---

### (b-1) Exit code em `powershell -Command` com binario nativo -- A CONFIRMAR

**Documentado para PS 7** (`about_Pwsh`, learn.microsoft.com): exit do processo e baseado em `$?` (bool), nao em `$LASTEXITCODE`. Binario nativo exit 2 -> `$?=$false` -> pwsh sai com 1.

**PS 5.1:** nao foi encontrada declaracao primaria explicita. Hipotese ate ML-0D.

**Sufixos cross-shell (medido 2026-10-04):**

```
$ bash -c '(exit 2); exit $LASTEXITCODE'; echo $?  -> 2
$ sh   -c '(exit 2); exit $LASTEXITCODE'; echo $?  -> 2
$ zsh  -c '(exit 2); exit $LASTEXITCODE'; echo $?  -> 2
```

Em bash/sh/zsh: `$LASTEXITCODE` indefinida -> empty word -> `exit` bare -> status do ultimo comando. O sufixo `; exit $LASTEXITCODE` preserva exit code em shells POSIX.

```
$ bash -c '(exit 0); if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }'; echo $?  -> 2
$ bash -c '(exit 2); if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }'; echo $?  -> 2
```

Sufixo do Gemini causa syntax error em bash; exit 2 e do proprio bash (guard NAO executa). PS-only. **medido**.

**cmd.exe -- `;` e separador de tokens [inferido, sem fonte primaria encontrada]:**

Em cmd.exe, `;` e tratado como separador de palavras, nao como operador de sequenciamento (`&` e o separador de comandos). `trackfw guard git-branch; exit $LASTEXITCODE` em cmd.exe produziria tokens `guard`, `git-branch`, ou `git-branch;` (comportamento exato de tokenizacao nao confirmado por fonte primaria). De qualquer forma, cobra recebe tokens extras e exibe help ou retorna 0.

Proxy medido (cobra local):

```
$ trackfw roadmap 'move;' exit '$LASTEXITCODE'; echo exit:$?
[help do subcomando roadmap]
exit:0
```

**medido** -- subcomando desconhecido exibe help e sai 0. Proxy para `guard` (que nao existe); gate ML-1A: `trackfw guard <qualquer-token-invalido>` deve sair 2, nao 0.

**Segundo risco do sufixo via canal argv (medido no .sh):** se o sufixo `;` for emitido para cmd.exe, cobra recebe os tokens como argv. `argv_overrides_stdin:0` demonstra no .sh que argv tem precedencia sobre stdin. CMD_RAW = "exit $LASTEXITCODE" (ou "exit" + "$LASTEXITCODE" dependendo de tokenizacao) nao e git bloqueado -> permite. Dois caminhos de fail-open para o mesmo sufixo em cmd.exe: cobra help (FN via subcomando invalido) e argv channel (FN via CMD_RAW permitido).

**Consequencia para D2:** CLIs PS/POSIX = `trackfw guard git-branch; exit $LASTEXITCODE`; CLIs cmd.exe = `trackfw guard git-branch` (sem sufixo). D2 sobrevive por linha de hook por CLI, nao como string universal. Se isso satisfaz o enunciado literal de D2 e decisao do arquiteto.

**CLIs impactados (se conversao confirmada):**

| CLI | Sufixo | Exit 2 via PS | Risco |
|---|---|---|---|
| Codex | SIM | 1 (hipotese) | fail-open ate ML-2A |
| Gemini | SIM (hardcoded, PS-only, correto) | 2 | correto |
| Windsurf | SIM | 1 (hipotese) | fail-open ate ML-2A |
| Claude Code PS fallback | SIM | 1 (hipotese) | fail-open ate ML-2A |
| Cursor | SIM | 1 (hipotese) | fail-open ate ML-2A |
| Kiro (cmd.exe) | NAO | 2 (direto) | correto |
| Amazon Q (cmd.exe) | NAO | 2 (direto) | correto |
| Copilot | NAO | 1 | correto (deny-all) |

**Experimento para ML-0D:**
1. `probe.exe` que le stdin (sha256 stderr), sai exit 2.
2. `cmd /c powershell.exe -NoProfile -Command "probe.exe"` -> `%ERRORLEVEL%`.
3. `cmd /c powershell.exe -NoProfile -Command "probe.exe; exit $LASTEXITCODE"` -> `%ERRORLEVEL%`.
4. `cmd /c "probe.exe; exit %ERRORLEVEL%"` (variavel cmd correta, nao `$`) -> verificar tokenizacao.
5. Repetir formas exatas dos CLIs.
6. Stdin: payload com `\u0000` e non-ASCII.
7. Braco npm: `Get-Item "$(npm prefix -g)\bin\trackfw.ps1" -Stream Zone.Identifier`.
8. Braco argv-cmd: `probe.exe git-branch; exit %ERRORLEVEL%` -- verificar quantos e quais argumentos `probe.exe` recebe (confirma hipotese de tokenizacao).

**Veredito:** condicional -- ML-0D decide. Se confirmado, ML-2A emite `; exit $LASTEXITCODE` para CLIs PS e linha limpa para cmd.exe.

---

### (b-2) Binario sem o subcomando `guard`

**Medido:**

```
$ trackfw guard git-branch </dev/null 2>&1; echo $?
Error: unknown command "guard" for "trackfw"
Run 'trackfw --help' for usage.
1

$ trackfw guard --help >/dev/null 2>&1; echo $?
1
```

**medido** -- v9.2.0 via Homebrew.

**Gate pre-ML-2A:**

```
$ printf '{"tool_input":{"command":"git push origin main"}}' \
    | bash scripts/trackfw-git-branch-guard.sh; echo $?
[deny JSON + REASON]
2
```

**medido** -- payload de deny -> exit 2.

**Lockout de ML-2A:** guard sai exit 2 em toda chamada Bash se binario nao tiver `guard` -> agente nao consegue usar o shell. Gate deve passar antes de despachar ML-2A.

**FP para Kiro e Copilot:** binario velho sai exit 1 -> deny-all ate binario atualizado.

**String fail-closed universal impossivel:** `|| exit 2` nao funciona em PS 5.1 (documentado).

---

### (c) Sequestro de PATH -- Aumento estrito

**Modelo antigo:** hook = caminho absoluto; sem dependencia de PATH no caminho de decisao (44+8 ocorrencias -- grep -o, medido, reproduz ADR -- todas em strings).

**Modelo novo:** hook = `trackfw guard ...`. Dependencia de PATH nova.

**cmd.exe cwd-before-PATH:**

```
$ ls trackfw.exe trackfw.cmd trackfw.bat trackfw.com 2>&1; echo found:$?
"trackfw.exe": No such file or directory (os error 2)
"trackfw.cmd": No such file or directory (os error 2)
"trackfw.bat": No such file or directory (os error 2)
"trackfw.com": No such file or directory (os error 2)
found:2
```

**medido** -- nenhum existe agora.

---

### (d) `trackfw` ausente do PATH

**macOS GUI:**

```
$ launchctl getenv PATH; echo rc=$?
rc=0
```

**medido** -- PATH ausente no launchctl. Apps GUI recebem `/usr/bin:/bin:/usr/sbin:/sbin`.

[inferido] Claude Code como app GUI -> exit 127 -> fail-open.

[inferido] CLIs VS Code (Cursor, Windsurf) no macOS: VS Code resolve o login shell ao iniciar.

**Caso medido -- Git Bash na VM Windows (ML-0B):** `~/bin` precede canais atualizados, com binario 8.0.0-rc2 sem `guard` -> exit 1 -> fail-open.

**Mitigacao:** `validate` versao >= piso minimo. Entra em ML-2B.

**Severidade: alta.**

---

### (e) cwd -- Dois guards, comportamentos DIFERENTES

**git-branch guard -- walk-up (medido L96-130):** `pwd -P` + loop de subida. Go deve replicar.

**credential guard -- cwd-only (medido L8):** `[ -f "trackfw.yaml" ] || exit 0`. Portar com walk-up mudaria comportamento -- nao autorizado.

**Credential guard: `INPUT=$(cat)` (L5) antes do `[ -f "trackfw.yaml" ]` (L8):** o guard le todo o stdin ANTES de verificar se esta num projeto trackfw. O Go deve preservar esta ordem (ou declarar a mudanca explicitamente).

**Risco de falsa heranca:** ML-1B deve ter busca de raiz independente (cwd-only) e sua propria logica de leitura de stdin.

---

### (f) Parser JSON em Go

**F1 -- Struct case-insensitive (fail-open medido):**

```
input: {"tool_input":{"command":"git push origin main","Command":"echo ok"}}
p.ToolInput.Command -> "echo ok"
```

**medido** -- jq DENY; Go struct ALLOW. Mitigacao: `map[string]json.RawMessage`.

**F2 -- Duplicate key last-wins (medido):**

```
input: {"command":"git push origin main","command":"echo ok"}
m["command"] -> "echo ok"
```

**medido**. ADR D2-ter confirmado.

**F3 -- NUL preservado (medido):**

```
input: {"command":"git push\u0000origin main"} -> len=20, cmd[8]==0
```

**medido**. Verificacao explicita obrigatoria (ADR D2-bis).

**F4 -- BOM:** PowerShell pode emitir BOM UTF-8. `json.Unmarshal` com BOM falha -> deve ser erro nomeado.

**F5 -- Canais de entrada e semantica do loop:**

Nao usar `io.ReadAll` simples para git-branch guard Go. O `.sh` usa um loop de chunks de 2s (L86-101); comportamento medido:

- Stdin ocioso 2s -> TRUNCATED -> nega (medido: idle:2 em 2s)
- Payload completo + pipe fecha DENTRO da janela de 2s corrente -> permite (medido: complete_noeof:0 com sleep 3: EOF em t=3s dentro da segunda janela que comecou em t=2s)
- Payload completo + pipe fecha DEPOIS da janela de 2s corrente **medido (late_eof_benign:2, late_eof_deny:2 com sleep 6)**: segunda janela inicia em t=2s, expira em t=4s com chunk vazio -> TRUNCATED -> nega -- FP para comando benigno
- Payload parcial + EOF -> nega por parse error (medido: partial:2 em 3s)
- argv presente: IGNORA stdin; CMD_RAW = "$*" (medido: argv_overrides_stdin:0)

**R9 -- stdin encoding e delivery:** payload com BOM, UTF-16, NUL; verificar se PS fecha o pipe dentro da janela de 2s corrente (se nao fechar: guard nega apos ~4s -- FP lockout medido com sleep 6). ML-0D inclui.

**Credential guard -- hang ate EOF (medido 2026-10-04):**

```
$ time (printf '{"tool_input":{"command":"echo ok"}}'; sleep 6) | bash scripts/trackfw-credential-guard.sh; echo cred_late:$?
(sem saida; exit 0) -- 6.046s total
cred_late:0
```

`INPUT=$(cat)` aguarda EOF sem timeout. Hang medido a 6s -> exit 0. **FN condicional:** se o CLI disparar seu proprio timeout e fizer allow, o guard nao bloqueou. Copilot documentado (fail-closed exceto timeout); demais CLIs nao determinados (item ML-0D). O mesmo vetor de entrega (pipe nao fechado) que causa FP no git-branch guard (~4s, nega benigno) causa FN condicional no credential guard (6s, permite se CLI faz allow no timeout). Decisao arquitetural: porta fiel (`cat`, sem timeout) com residuo declarado, ou adiciona timeout como mudanca de comportamento autorizada.

Credential guard Go: preservar a ordem ler-antes-de-checar-projeto (L5 antes de L8).

---

## Secao 3 -- Alvos de Falsificacao nas Duas Direcoes

**FN:** guard aceita quando deveria negar. **FP:** guard nega quando deveria aceitar.

### Guard Go (`internal/guard/`)

| Superficie | Vetor FN | Gate FN | Vetor FP |
|---|---|---|---|
| Struct case-insensitive | `{"tool_input":{"command":"git push","Command":"echo ok"}}` -> Go ALLOW, jq DENY | Fixture F1; ambos devem DENY | -- |
| NUL sem verificacao | payload com `\u0000` | Fixture D2-bis; guard nega | -- |
| BOM | -- | -- | BOM -> erro nomeado, nao deny-all |
| Duplicate key first-wins (erro) | `{"command":"echo ok","command":"git push"}` -> first-wins ALLOW | Fixture F2; last-wins DENY | -- |
| Walk-up herdado por credential | credential nega em subdiretorio | Teste em subdiretorio: credential no-op | -- |
| argv ignora stdin bloqueado | `printf DENY_PAYLOAD \| guard git-branch BENIGN_CMD` -> exit 0 | **Medido no .sh (argv_overrides_stdin:0)**: gate ML-1A: em modo hook, guard ignora argv OU valida que linha de hook nao contem tokens apos a folha | -- |
| argv com --force (flag desconhecida no cobra) | cobra: "unknown flag" -> exit 1 -> fail-open (6 CLIs) | Gate ML-1A: todo erro cobra sob `guard` sai 2 com deny JSON | -- |
| Stdin ocioso sem timeout (git-branch) | stdin open sem dados -> hang -> CLI timeout -> fail-open | Teste: `(sleep 5) \| trackfw guard git-branch` -> exit 2 em < 4s | -- |
| Pipe fecha fora da janela de 2s corrente (git-branch) | -- | -- | FP medido: late_eof_benign:2 a ~4s (sleep 6; bash 5.3.20); gate ML-1A: `(payload; sleep 3) \| guard` -> allow; `(payload; sleep 6) \| guard` -> deny ~4s |
| Credential guard hang ate EOF -- FN condicional (hang medido) | `(printf payload; sleep 6) \| credential` -> exit 0 a 6s (cred_late:0). FN condicional: se CLI timeout faz allow, guard nao bloqueou. Copilot documentado; outros CLIs nao determinados (item ML-0D). | Teste: hook com stdin nunca fechado -> guard sai antes do timeout do CLI (decisao arquitetural: porta fiel vs. adicao de timeout) | -- |
| Leitura-apos-verificacao no credential Go -- FP potencial | -- | Teste: credential fora de projeto; se Go verificar projeto antes de ler stdin, EPIPE no escritor -> CLI pode falhar unexpectedly | Go deve ler stdin antes de verificar projeto (ordem de L5/L8 do .sh) |
| Prioridade de chaves incorreta | `tool_info.command_line` preferida | Fixture com ambos; usa `tool_input.command` | -- |
| Dois objetos JSON | payload com dois objetos | Falhar-fechado | -- |
| TRACKFW_GIT_COMMAND ignorada | env var fallback nao implementada | Teste: env var -> exit 2 para bloqueado | -- |

### Involucro `.sh` (D3 -- apos ML-2A)

| Superficie | FN | Gate | FP |
|---|---|---|---|
| Sem verificacao de subcomando | binario 9.2.0 -> exit 1 | Involucro testa `trackfw guard --help`; sem subcomando -> exit 2 | -- |
| Script adulterado | passa | `*_script_integrity` acusa | -- |

### Emissao por CLI (`internal/generators/agentfiles.go`)

| Superficie | FN | Gate | FP |
|---|---|---|---|
| Sufixo `; exit $LASTEXITCODE` emitido para Kiro/Amazon Q | (i) cobra help -> exit 0 -> fail-open [proxy medido]; (ii) argv channel: CMD_RAW="exit $LASTEXITCODE" -> allow [medido no .sh] | Gate ML-1A-i: `trackfw guard <folha-invalida>` -> exit 2. Gate ML-1A-ii: todo erro cobra sob `guard` -> exit 2 com deny JSON | -- |
| Codex/Windsurf sem sufixo (se ML-0D confirmar) | PS converte exit 2 -> 1 -> fail-open | ML-0D + teste que afirma presenca do sufixo | -- |
| Sufixo PS-only emitido para CLI bash | -- | -- | bash syntax error -> bash exit 2 -> deny-all [medido] |
| Copilot sem campo `command` (ADR D4) | hook ausente | Campo `command` presente | -- |
| `migrateHookCommand` nao migra string antiga | config legada | Teste ida e volta | -- |
| `check-git-branch-guard-hook-schema.sh` nao atualizado no ML-2A | CI testa `.sh` sem binario | Falha CI detectavel | -- |

### Validate (ML-2B)

`collectCommandsWithMarker` usa `strings.Contains` (substring match, **medido em L282, L290** de `validator_credential_guard.go`). Depois da migracao, a linha de hook e `trackfw guard git-branch` (ou com sufixo). Um hook como `trackfw guard git-branch --verbose` ou `trackfw guard git-branch; exit $LASTEXITCODE` passaria validate mesmo se o novo marker for a string base -- pois ela esta contida como substring. Isso e a mesma classe do achado de dedup do guard (nota `project_guard_dedup_hook_structure_gap`).

Gate ML-2B: validate deve comparar a linha de hook EXATAMENTE (ou via regex que inclui o sufixo permitido), nao como substring.

### ML-1C -- Risco de vacuidade pos-ML-2A

Apos ML-2A converter `gitBranchGuardScript` em `exec trackfw guard ...`, ML-1C compararia Go com Go. Mitigacao: congelar as 756 linhas do `.sh` como fixture hash-pinned em testdata ANTES do ML-2A.

---

## Secao 4 -- Residuo Declarado e Vereditos por Risco D5

| Risco | Falha | Severidade | Veredito | ML |
|---|---|---|---|---|
| (a) npm+PS+Restricted | 3 CLIs fail-open; 1 deny-all | Alta | validate orienta (se sem MoTW) ou KG declara unsupported; ML-0D verifica | ML-0D + ML-2B |
| (b-1) Exit code PS 5.1 | A confirmar | Potencialmente critica | ML-0D decide; se confirmado, ML-2A emite `; exit $LASTEXITCODE` para CLIs PS, linha limpa para cmd.exe | ML-0D + ML-2A |
| (b-2) Binario sem `guard` | 6/8 CLIs fail-open; Kiro/Copilot deny-all | Alta | involucro `.sh` fail-closed + validate versao; gate pre-ML-2A | ML-2A + ML-2B |
| (c) Sequestro de PATH | Aumento estrito (nova dependencia) | Alta | validate nova regra (trackfw.exe/cmd na raiz) | novo ML |
| (d) trackfw ausente do PATH | Fail-open (exit 1 medido, exit 127 inferido) | Alta | validate detecta resolucao + versao | ML-2B |
| (e) cwd | git-branch: walk-up; credential: cwd-only; credential le stdin antes de checar projeto | Media | ML-1A: walk-up + semantica de loop; ML-1B: cwd-only + `cat` antes do check; funcoes independentes | ML-1A + ML-1B |
| (f) Parser JSON + canais | F1 fail-open; F5 loop (late_eof FP medido); F3/F4 varios; argv-overrides-stdin FN; credential hang FN medido | Media | `map[string]json.RawMessage`; loop de 2s git-branch; porta fiel ou timeout declarado p/ credential; decisao de argv em hook mode | ML-1A + ML-1B |

**Decisao do arquiteto necessaria:** argv em hook mode. Opcoes: (i) modo hook proibe tokens apos a folha e validate afirma linha exata; (ii) argv desabilitado em modo hook com `--command` flag. Gate ML-2B acompanha.

**Residuos permanentes:**

| # | Residuo | Dependencia |
|---|---|---|
| R1 | Exit code PS 5.1 nao medido | ML-0D resolve |
| R2 | npm+PS+Restricted: decisao unsupported cabe ao KG se MoTW presente | ML-0D + KG |
| R3 | GUI macOS sem PATH [inferido] | Estrutural |
| R4 | Cursor e Amazon Q -- cwd nao determinado | ML-0C |
| R5 | Git Bash com binario velho em `~/bin` | ML-0B |
| R6 | `globalCredentialGuardScript` sem porta explicita | Decisao antes ML-1B |
| R7 | cmd.exe cwd-before-PATH: nova regra de `validate` | novo ML |
| R8 | Evasoes no cabecalho do `.sh` portadas sem fechar | ADR-2026-08-12 |
| R9 | stdin delivery: BOM, UTF-16, NUL. git-branch: PS nao fecha pipe dentro da janela de 2s corrente -> FP lockout medido (~4s, bash 5.3.20). credential: CLI nunca fecha stdin -> FN hang medido (6s, exit 0) -> FN condicional a politica de timeout do CLI (Copilot documentado; demais nao determinados). Dois guards, direcoes opostas. Decisao de timeout e arquitetural. | ML-0D + decisao arquitetural |

---

### Checklist de aceite do ML-0A (AC da linha 113-116 do roadmap)

- [x] AC1: As quatro secoes com evidencia (comando e saida) -- Secao 1 (enumeracao), Secao 2 (threat model), Secao 3 (falsificacao), Secao 4 (residuo)
- [x] AC2: Lista de sitios fechada, com veredito por sitio -- Secao 1 (todos os arquivos e canais enumerados + adicoes ao escopo declarado)
- [x] AC3: Um veredito por risco da D5: "bloqueia o desenho" ou "mitigacao X, entra no ML Y" -- Secao 4, tabela de riscos (a)-(f) e residuos R1-R9
- [x] Nenhuma linha de codigo de produto alterada

**Nota para o arquiteto:** marcar o ML-0A como ✅ Concluido no roadmap e os `[x]` dos AC acima e responsabilidade do arquiteto (hades-tf nao tem autoridade de git). O gate da barreira verifica os marcadores ✅ e AC `[x]`, nao a qualidade da analise -- a auditoria do conteudo cabe ao arquiteto.

---

*Documento produzido por hades-tf (Hades, Security Reviewer) em 2026-10-04.*
*Nenhum codigo de produto foi alterado.*
