# Red-team ML-2B — Marcador do guard fora de hook (D16, REQ-2026-10-25)

**Data:** 2026-10-25
**Agente:** hades-tf
**Branch:** fix/validate-marcador-do-guard-fora-de-hook
**Binário medido:** trackfw 11.0.1 (compilado desta branch)
**Wave 0:** `docs/seguranca/2026-10-25-wave0-marcador-fora-de-hook.md`

---

## Sumário executivo

A implementação do ML-1A fecha os fail-opens documentados na Wave 0 para os vetores de campo
auxiliar (`"note"`, `"autoMode.environment"`, `"permissions.allow"`). O caso original #618 está
corrigido. Dois bloqueantes foram encontrados — nenhum é regressão de D16, mas ambos existem na
branch que este PR cierra e, sob a Regra Dura de Causa Raiz, pertencem à mesma causa: o validador
conta como fiação um marcador que o CLI não executa.

**Veredito: REPROVA — 2 bloqueantes (B1, B2).**

---

## 1 — Binário e ambiente

```
B=$(mktemp -d) && go build -o $B/trackfw ./cmd/trackfw
$B/trackfw --version  # → trackfw 11.0.1
```

Cada fixture: `T=$(mktemp -d) && mkdir -p "$T/home" "$T/proj" && cd "$T/proj" && test ! -e go.mod || exit 1`
Validação: `HOME="$T/home" $B/trackfw validate`

---

## 2 — Tabela de ataques

| # | Vetor | Resultado | rc | Evidência (ver §3) |
|---|---|---|---|---|
| 1a | Claude: hook sem `type:"command"` | PASS | 1 | violation: "missing type:command" |
| 1b | Claude: `command` no nível de grupo (`PreToolUse[i].command`), sem inner `hooks[]` | PASS | 0 | collector ignora (sem presença rule para Claude); comportamento igual com ou sem D16 |
| 1c | Claude: `command` em objeto não-hook (`metadata`) | PASS | 0 | collector ignora; sem false positive |
| 1d | Claude: `command` é array em vez de string | PASS | 0 | `obj["command"].(string)` falha em type assert; ignorado |
| 1e | Claude: chaves `command` duplicadas no JSON (guard first, `echo x` last) | PASS | 0 | Go last-key-wins: `echo x` é o valor final; collector não encontra guard; sem false positive |
| 2a | Windsurf projeto: marcador exato em `"note"`, `command:"echo placeholder"` | **PASS** | 1 | 2 violations: "no credential guard entry in event pre_run_command/pre_write_code" |
| 2b | Windsurf projeto: subcmd marker em `"note"`, `command:"echo placeholder"` | **PASS** | 1 | idem |
| 2c | Amazon Q: subcmd marker em `"note"` dentro de `hooks[]`, `command:"echo x"` | **PASS** | 1 | 2 violations: "no credential guard entry in event preToolUse[execute_bash/fs_write]" |
| 2d | Kiro: marcador em `action.name`, `action.command:"echo x"` | **PASS** | 0 | collector ignora; sem false positive (Kiro não tem presence rule) |
| 3a | Copilot global: `"bash"` key com guard (forma correta) | PASS | 1 | form violation (forma desatualizada); guard detectado |
| 3b | Copilot global: somente `"powershell"` key, sem `"bash"` | R | 0 | `hookCmdSchemaFlatBash` lê só `"bash"`; ver **R1** |
| 4a | Windsurf global: `"command"` key com guard (forma correta) | PASS | 1 | form violation; guard detectado |
| 4b | Windsurf global: guard somente em `"note"` | PASS | 0 | collector ignora; sem false positive (global sem presence rule para o evento) |
| 5a | Claude: guard em `PostToolUse` com forma canônica atual | **FAIL → B1** | 0 | rc=0; guard encontrado mas em evento não-bloqueante; §3-B1 |
| 5b | Kiro: guard em `onAfterToolUse` com forma Kiro canônica | **FAIL → B1** | 0 | rc=0; idem |
| 5c | Windsurf projeto: guard em `post_write_code` (evento não-requerido) | PASS | 1 | presence rule checa eventos específicos; rc=1 |
| 5d | Claude: `command` em chave desconhecida `preHookInit` (dentro de `hooks[]`) | R | 0 | campo desconhecido ignorado por D16; ver **R2** |
| 6a | `autoMode.environment` com `.sh` marker e hook correto (#618) | PASS | 0 | zero warnings legados; só form violation pelo hook |
| 6b | `permissions.allow` com subcmd marker e hook correto | PASS | 1 | somente form violation pelo hook; nenhuma violation pelo `permissions.allow` |

---

## 3 — Evidência dos bloqueantes

### B1 — Guard em evento não-bloqueante passa validate (CLIs sem presence rule por evento)

**Causa:** `collectCommandsWithMarkerBySchema` itera `for _, evRaw := range hooksObj` — coleta de
TODOS os eventos. Para CLIs com presence rules por evento (Windsurf `pre_run_command/pre_write_code`,
Amazon Q `execute_bash/fs_write`), o check correto existe. Para CLIs sem presence rule por evento
(Claude Code, Codex, Gemini, Cursor, Copilot global, Kiro), um guard na forma canônica em qualquer
evento passa validate inteiramente.

**Medição:**

```
T=$(mktemp -d) && mkdir -p "$T/home" "$T/proj" && cd "$T/proj"
git init -q && git config user.email t@t.com && git config user.name t
mkdir -p .claude
CANONICAL='emulate -R sh 2>${null-/dev/null}; $LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard credential; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE'
printf '{"hooks":{"PostToolUse":[{"matcher":"Bash|PowerShell","hooks":[{"type":"command","command":"'"$CANONICAL"'"}]}]}}' > .claude/settings.json
out=$(HOME="$T/home" $B/trackfw validate 2>&1); rc=$?
printf '%s\n' "$out"
echo "rc=$rc"
```

**Resultado:**
```
⚠  adr_dir "docs/adr" does not exist
⚠  write_scope_not_wired: .claude/settings.json PreToolUse[Write|Edit|NotebookEdit] missing ...
2 warning(s)
rc=0
```

Claude Code não executa hooks em `PostToolUse` para bloquear ferramentas. O guard nunca roda.
Validate diz: nenhum problema.

Mesmo resultado para Kiro com `onAfterToolUse` e forma canônica `trackfw guard credential || exit 2`:
```
2 warning(s)
rc=0
```

**Controle:** Windsurf projeto com guard em `post_write_code` (evento não-requerido):
```
✗ .windsurf/hooks.json (Windsurf) is present but has no credential guard entry in event pre_run_command
✗ .windsurf/hooks.json (Windsurf) is present but has no credential guard entry in event pre_write_code
Error: 2 violation(s) found
rc=1
```

Windsurf funciona porque `validateCredentialGuardPresenceRequired` tem a lista de eventos esperados
(`pre_run_command`, `pre_write_code`). Claude e Kiro não têm equivalente.

**Direção:** fail-open. A mesma causa do campo errado (marcador onde o CLI não executa) repetida no
eixo de evento em vez de campo. Regra Dura de Causa Raiz: pertence a esta REQ.

**Fix mínimo:** expandir `validateGuardHookResolvable` (e equivalente global) para rejeitar
matches em eventos fora da lista esperada por CLI, OR adicionar a lista de eventos esperados ao
`credentialGuardHookFile` struct e verificar antes de aceitar o match. Não é mudança de D16 em si
— D16 restringiu campo, o fix aqui restringe evento.

### B2 — Guard em `PreToolUse` com nível de aninhamento errado (group level) não gera violation

**Contexto:** Attack 1b — `command` no nível do grupo (`PreToolUse[i].command`, com `type:"command"`
no mesmo objeto), sem o array `hooks[]` interno. Claude Code não executa este formato.

**Medição:**

```
mkdir -p .claude
printf '{"hooks":{"PreToolUse":[{"matcher":"Bash|PowerShell","type":"command","command":"trackfw guard credential || exit 2; exit $LASTEXITCODE"}]}}' > .claude/settings.json
out=$(HOME="$T/home" $B/trackfw validate 2>&1); rc=$?
printf '%s\n' "$out"
echo "rc=$rc"
```

**Resultado:**
```
⚠  adr_dir "docs/adr" does not exist
⚠  write_scope_not_wired: .claude/settings.json PreToolUse[Write|Edit|NotebookEdit] ...
2 warning(s)
rc=0
```

**Análise:** `hookCmdSchemaNested` requer `group["hooks"][j]["command"]`. O formato errado tem
`group["command"]` (sem array `hooks`). O collector não encontra o marcador na posição correta →
nenhuma violation é emitida. Claude Code não executa o formato incorreto → guard silenciosamente
ausente.

**Diferença do B1:** neste caso o usuário ficou com rc=0 acreditando que instalou o guard, mas a
estrutura está incorreta e Claude ignora o hook. Em B1, a estrutura está correta mas o evento é
errado.

**Classificação:** este comportamento é idêntico ao de origin/main (ambos ignoram um hook mal
estruturado). O D16 não o introduziu nem o piorou. Contudo, a causa é a mesma que B1 ("marcador
onde o CLI não executa") e a presença de B2 em origin/main não move este achado para R — a correção
pertence ao mesmo PR.

**Nota de escopo:** Claude Code não tem presence rule por projeto (diferente de Windsurf). A
correção de B2 pode ser feita como um aviso (warning) de estrutura incorreta no `validateGuardHookResolvable`,
sem exigir presence rule completa.

---

## 4 — Evidência dos PASS selecionados

### AC4 (Windsurf projeto, subcmd marker em note):
```
✗ .windsurf/hooks.json (Windsurf) is present but has no credential guard entry in event pre_run_command
✗ .windsurf/hooks.json (Windsurf) is present but has no credential guard entry in event pre_write_code
Error: 2 violation(s) found
rc=1
```

### AC4 (Amazon Q, subcmd marker em note dentro de inner hooks[]):
```
✗ .amazonq/cli-agents/q_cli_default.json (Amazon Q) is present but has no credential guard entry in event preToolUse[execute_bash]
✗ .amazonq/cli-agents/q_cli_default.json (Amazon Q) is present but has no credential guard entry in event preToolUse[fs_write]
Error: 2 violation(s) found
rc=1
```

### AC3 (#618 — autoMode.environment com hook correto):
```
✗ ~/.claude/settings.json (Claude Code, global scope) has "..." but the expected form...
Error: 1 violation(s) found
rc=1
```
Nenhuma warning de `.sh` legacy. O único error é a forma desatualizada do hook (correto).

### AC3 (permissions.allow com hook correto):
```
✗ .claude/settings.json (Claude Code) has "..." but the expected form...
Error: 1 violation(s) found
rc=1
```
Nenhuma violation extra pelo `permissions.allow`.

### Copilot global bash key detectado (3a):
```
✗ ~/.copilot/settings.json (GitHub Copilot CLI, global scope) has "..." but the expected form...
rc=1
```

### Windsurf global command key detectado (4a):
```
✗ ~/.codeium/windsurf/hooks.json (Windsurf, global scope) has "..." but the expected form...
rc=1
```

---

## 5 — Residuais declarados

**R1 — Copilot global: somente chave `"powershell"` (sem `"bash"`).**
O gerador (`mergeCredentialGuardCopilotHooks`) escreve apenas a chave `"bash"`. Um usuário que
edite manualmente o arquivo adicionando uma entrada com chave `"powershell"` (e omitindo `"bash"`)
ficará com o guard invisível ao validator. O Copilot CLI executa entries com `"bash"` key; não há
evidência de que execute via `"powershell"` key. Impacto: somente arquivos hand-crafted fora do
gerador. Declarado como residual (não produzido pelo gerador = fora do contrato D16).

**R2 — Campo futuro desconhecido em entrada de hook.**
Se um CLI futuro adicionar um segundo campo de comando (e.g., `"preHookInit"` ou `"script"`) e o
usuário colocar o guard nesse campo, o validator ignorará e emitirá violation de ausência (ou
nenhuma mensagem, se o CLI não tiver presence rule). Comportamento fail-closed na direção de
ausência. Declarado como residual de versão futura de CLI.

**R3 — Chaves `command` duplicadas no JSON: guard-last é detectado, guard-first não.**
Medição confirmou que Go's `json.Unmarshal` aplica last-key-wins. Se o guard é a primeira de duas
entradas `"command"` e `echo x` é a segunda, o collector vê apenas `echo x`. O inverso
(guard-last) é detectado corretamente. Risco prático baixo: duplicata de chave é JSON inválido e
nenhum gerador do trackfw a produz.

**R4 — Copilot global em `postToolUse` com forma canônica: rc=0.**
Copilot global tem a mesma ausência de presence rule que Claude Code. Um guard em `postToolUse`
com forma canônica não é bloqueante para Copilot e passa validate. Mesma causa que B1; a
classificação como R em vez de B independente justifica-se porque este arquivo é global e o gerador
escreve em `preToolUse`. Arquivos gerados pelo trackfw não sofrem B1.

---

## 6 — Achados de B1 e B2 versus origin/main

B1 e B2 **existiam em origin/main** (o scanner recursivo também encontrava marcadores em PostToolUse
e no nível de grupo). D16 não os introduziu. Contudo:

- D16 é a correção de causa raiz para "marcador onde o CLI não executa (eixo campo)".
- B1 e B2 são a mesma causa no eixo evento e no eixo profundidade de aninhamento.
- Sob a Regra Dura de Causa Raiz, mesma causa → mesmo roadmap → mesmo PR.
- Deixar B1/B2 no PR significa mergear com achados de causa raiz idêntica não corrigidos.

---

## 7 — git status

```
(saída limpa — nenhum arquivo modificado exceto este parecer)
```

Verificado com `git status --short --untracked-files=all` antes e após a escrita do parecer.
Resultado: output vazio.
