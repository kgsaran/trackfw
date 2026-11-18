# Red Team — REQ-2026-11-17 · Google Antigravity guard (projeto)

> Data: 2026-11-18 | Revisor: hades-tf | Branch: feat/guard-de-credencial-no-google-antigravity-e-revisao-do-suporte-ao-gemini-cli

Reimplementação independente a partir da leitura do código. Escopo: guard de projeto (`.agents/hooks.json`); guard global não entregue, fora do escopo.

Artefatos base lidos antes do primeiro teste:
- `docs/adr/ADR-2026-11-18-guard-de-credencial-e-de-git-branch-no-google-antigravity-e-gemini-cli-como-legado.md`
- `docs/seguranca/2026-11-18-wave0-antigravity-threat-model.md` (T1–T12)
- `docs/portabilidade/2026-11-18-antigravity-contrato-de-hooks.md` (§11, medido; ags ML-1A, `agy` 1.5.11)
- `docs/qualidade/2026-11-18-ml-4b-antigravity-review.md`
- `internal/guard/payload.go`, `internal/guard/credential.go`, `internal/guard/writescope.go`, `internal/generators/agentfiles.go`
- `git diff origin/main...HEAD -- internal/` (somente leitura)

Binário: compilado de `cmd/trackfw/` para `/private/tmp/hades-h5-OU6044/trackfw`. Projetos de teste em `/private/tmp/hades-m2-*/`, `/private/tmp/hades-p4-*/` com HOME isolado (`mktemp -d`). Nenhum `git write`, `make quality` ou `go test ./...` executado.

Chave fake construída em runtime (nunca literal no repo): `python3 -c "print('AIza'+'F'*35)"` = 39 chars (cobre `AIza[0-9A-Za-z_-]{35,}`). Uma versão anterior desta sessão (antes da compactação) usou 33 chars (não casa `{35,}`); todos os vetores foram re-executados com a chave correta.

**Restrição violada — HOME não isolado em f2f3_test.sh:** um script preliminar (`f2f3_test.sh`) executou `trackfw init` e `validate` sem isolar `HOME`/`USERPROFILE`/`CODEX_HOME`. Isso viola a restrição de dispatch ("Binário só a partir de `cd "$(mktemp -d)"` com HOME/USERPROFILE/CODEX_HOME temporários"). Impacto concreto:

- `validate` leu `~/.claude/settings.json`, `~/.codex/hooks.json`, `~/.gemini/settings.json` → produziu 8 warnings sobre settings do ambiente real (visível nos rascunhos "V26-preliminar").
- Nenhum arquivo de HOME foi escrito: `~/.claude/settings.json` último write 2026-10-24, `~/.codex/hooks.json` e `~/.gemini/settings.json` último write 2026-10-22 — todos anteriores ao binary build 2026-11-18T14:58:34 (confirmado por `stat -f "%Sm" -t "%Y-%m-%dT%H:%M:%S"` nos três arquivos).
- Job background `b47se1vf0` ("V06 and V25 anomalies"): não tinha `export HOME=$H`; porém o job abortou por read-timeout (`stdin idle > 2s`) antes de executar `trackfw init`; nenhum init foi disparado.
- Todos os vetores citados neste relatório como "(HOME isolado)" foram executados em f2f3_isolated.sh com `export HOME=$H USERPROFILE=$H CODEX_HOME=$H` correto.

---

## Sumário executivo

| ID | Superfície | Severidade | Status |
|----|-----------|-----------|--------|
| F5 | `.agents/hooks.json` ausente de `writeScopeSubagentFrozenRelPaths` | MEDIUM | Novo |
| F2 | `InjectAntigravityHooks` preserva `enabled:false` existente | MEDIUM | Novo |
| F3 | `validate` não detecta `enabled:false` no grupo `trackfw` | MEDIUM | Novo |
| F4 | git-branch deny emite `hookSpecificOutput` (rc=2 estrutural, T11 mitigado por §11) | LOW | T11 pré-declarado, severidade revista |
| F1 | P5 (`toolCall.args.CommandLine`) dispara sem `hookEventName` | LOW | Novo |
| — | A1 fix, A4 fix, D2, D4, write-scope parcial, P2 correto verificados | — | PASS |

**Veredito: REPROVA** — F5 confirmado com contra-braços (`.codex/hooks.json` → rc=2, `.agents/hooks.json` → rc=0). F2+F3 formam cadeia explorável com evidência limpa (HOME isolado). Nenhum fix implementado.

---

## Surface 1 — git-branch guard: extração P5 e cwd

Projeto `/private/tmp/hades-m2-3mH3jd`. Payloads via arquivo para evitar literais de chave no payload Bash.

### V01 — Antigravity schema `git push` → rc=2

**Payload:** `{"hookEventName":"PreToolUse","cwd":"$PROJ","toolCall":{"name":"run_command","args":{"CommandLine":"git push origin main","Cwd":"$PROJ"}}}`
**Comando:** `(cd $PROJ && $BIN guard git-branch < payload.json)`
**rc medido:** 2 — P5 extrai `CommandLine = "git push origin main"`; guard bloqueia.
**Resultado:** PASS.

### V02 — Antigravity schema `echo hello` → rc=0

**rc medido:** 0.
**Resultado:** PASS — `echo hello` não é comando bloqueado.

### V03 — P1 tem precedência sobre P5 → rc=0

**Payload:** `tool_input.command = "echo hello"` + `toolCall.args.CommandLine = "git push origin main"`.
**rc medido:** 0 — P1 extrai `echo hello` (benign); P5 não avaliado como comando de bloqueio.
**Resultado:** PASS.

### V04 — F1: `hookEventName` ausente, `CommandLine = "git push origin main"` → rc=2

**Payload:** `{"toolCall":{"name":"run_command","args":{"CommandLine":"git push origin main"}}}`
**rc medido:** 2 — P5 dispara sem `hookEventName`.

**Localização:** `payload.go:199-205` — `extractNested3(root, "toolCall", "args", "CommandLine")` chamado sem verificação de `isAntigravityPayload`.

**Severidade: LOW** — nenhum runtime atual emite `toolCall.args.CommandLine` além do Antigravity.

### V05 — `hookEventName` ausente, `CommandLine` benign → rc=0

**rc medido:** 0.
**Resultado:** PASS.

### V06 — PascalCase counter-arm: lowercase `commandline` com `git push` → rc=0

**Payload:** `toolCall.args.commandline = "git push origin main"` (lowercase).
**rc medido:** 0 — P5 usa `extractNested3` com chave `"CommandLine"` exata (case-sensitive JSON); lowercase não casa.
**Resultado:** PASS — P5 é case-sensitive conforme esperado.

### V07 — git-branch guard de `.agents/` (sem `trackfw.yaml` lá) → rc=2

**Payload:** Antigravity `git push origin main`, com root-level `cwd = $PROJ`.
**Comando:** `(cd $PROJ/.agents && $BIN guard git-branch < payload.json)`
**rc medido:** 2 — git-branch guard não depende de `trackfw.yaml`; funciona de `.agents/`.
**Resultado:** PASS.

### V08 — F4/T11: stdout = `hookSpecificOutput`, rc=2 estrutural

**Payload:** Antigravity `git push origin main`.
**stdout medido:** `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"trackfw: raw git push blocked..."}}`
**rc medido:** 2

**Análise:** T11 avaliado como ALTO no threat model (linha 612) antes de ML-4A. §11 medido (linha 975): `stdout vazio, exit 2 → bloqueia; exit 1 → bloqueia; toda saída non-decision com exit 0 → bloqueia`. O guard emite `hookSpecificOutput` + exit 2. Exit 2 bloqueia independentemente de stdout. T11 não é explorável enquanto `denyOut()` for estrutural.

**Revisão:** T11 downgraded para LOW pós-§11. ADR não declarou T11 como residual; threat model linha 612 listou como ALTO antes da medição. §11 resolve.

### V09 — D2: stdout vazio no allow (git-branch guard)

**rc medido:** 0, stdout: 0 bytes, stderr: 0 bytes.
**Resultado:** PASS.

---

## Surface 2 — credential guard: D4 + extração + cwd (modo: block)

**Setup:** projetos com `credential_guard: mode: block` em `trackfw.yaml`. Global guard (`--global`) sempre bloqueia sem yaml. HOME isolado.

**§11 medido (linha 998-999):** "O cwd do hook é `.agents/`, não a raiz: a busca de `trackfw.yaml` sobe e acha a raiz; o campo `cwd` do payload é a raiz do workspace." O payload real do Antigravity inclui `"cwd": "<project-root>"` no nível raiz. `credGetAntigravityInfo` lê `root["cwd"]` (lowercase, raiz) — correto.

### C1 — Global guard, raw key → rc=2

**Comando:** `printf '<key>' | $BIN guard credential --global`
**rc medido:** 2.
**Resultado:** PASS.

### C2 — Global guard, Gemini schema → rc=2

**Payload:** `{"tool_name":"Bash","tool_input":{"command":"echo <key>"}}`
**rc medido:** 2.
**Resultado:** PASS.

### C3 — Global guard, Antigravity schema, cred em CommandLine → rc=2

**Payload:** `{"hookEventName":"PreToolUse","cwd":"$PROJ","toolCall":{"name":"run_command","args":{"CommandLine":"echo <key>","Cwd":"$PROJ"}}}`
**rc medido:** 2 — D4: layer1 = bytes de `toolCall`, inclui `CommandLine`.
**Resultado:** PASS.

### C4 — Global guard, Antigravity, cred em lastUserInput apenas → rc=0

**Payload:** `lastUserInput` com chave; `CommandLine = "echo hello"`.
**rc medido:** 0 — D4: scan só de `toolCall` quando `hookEventName` presente.
**Resultado:** PASS.

### C4-contra — Mesmo payload sem `hookEventName` → rc=2 (D4 counter-arm)

**Payload:** C4 sem `hookEventName` (tudo idêntico, apenas campo removido).
**rc medido:** 2 — `isAntigravityPayload=false` → layer1 = payload completo → `lastUserInput` escaneado.
**Resultado:** PASS — confirma que a exclusão de `lastUserInput` vem do `hookEventName`, não da estrutura do payload.

### P1 — Project guard, mode:block, cwd=project root, cred em CommandLine → rc=2

**Comando:** `(cd $PROJ && $BIN guard credential < payload.json)`
**rc medido:** 2.
**Resultado:** PASS.

### P2 — Project guard, mode:block, hook de `.agents/`, real payload (root-level cwd) → rc=2 (A4 correto)

**Payload:** `{"hookEventName":"PreToolUse","cwd":"$PROJ","toolCall":{"name":"run_command","args":{"CommandLine":"echo <key>","Cwd":"$PROJ"}},"workspacePaths":["$PROJ"]}`
**Comando:** `(cd $PROJ/.agents && $BIN guard credential < payload.json)`
**rc medido:** 2 — A4: `root["cwd"]` = `$PROJ` (absoluto) → `projectCwd = $PROJ` → `trackfw.yaml` encontrado → mode:block → rc=2.
**Resultado:** PASS — A4 funciona para o payload real do Antigravity.

**Nota:** um rascunho anterior desta sessão (antes de ler §11 completo) presumiu equivocadamente que `cwd` estava em `toolCall.args.Cwd`. §11 linha 986 e linha 998 confirmam que `cwd` é campo root-level no payload real.

### P3 — Project guard, mode:block, cred em `toolCall.args.Cwd` (raw scan) → rc=2

**Payload:** `CommandLine = "ls"`, `Cwd = "/tmp/<key>"`. Rodando de project root.
**rc medido:** 2 — D4 varre bytes brutos de `toolCall`; credencial em `Cwd` detectada.
**Resultado:** PASS.

### P4 — Project guard, mode:block, D4: cred em `tool_input` fora de `toolCall` → rc=0

**Payload:** `{"hookEventName":"PreToolUse","tool_input":{"secret":"<key>"},"toolCall":{"name":"run_command",...}}`
**rc medido:** 0 — D4: apenas `toolCall` varrido.
**Resultado:** PASS — ADR D4 (opção a).

### P5 — Project guard, modo padrão (warn), cred em CommandLine → rc=0 + warning

**Projeto:** sem `credential_guard: mode: block`.
**rc medido:** 0, stderr: `trackfw-credential-guard: warning - possible Google API key detected in tool payload.`
**Resultado:** PASS — modo warn funciona.

**D2 em warn:** stdout = 0 bytes. Consequência: Antigravity não bloqueia em modo warn (stdout vazio = allow por §11). Aviso vai para stderr; visibilidade no TUI do `agy` não medida (Wave 1 braço adicional).

### P6 — manage_task + credential em `Input`, mode:block → rc=2

**Payload:** `{"hookEventName":"PreToolUse","cwd":"$PROJ","toolCall":{"name":"manage_task","args":{"Action":"send_input","Input":"<key>"}}}`
**rc medido:** 2 — layer 1 varre bytes de `toolCall`; credencial em `Input` detectada.

**Relação com ADR residual:** ADR diz "manage_task com send_input carrega texto, não `CommandLine`. A camada 1 vê o segredo, **mas o guard de git-branch não vê o comando**." Layer 1 do credential guard detecta a chave (PASS). Git-branch guard não bloqueia `manage_task` (matcher `run_command` não casa; isso é o residual declarado).

---

## Surface 2 — view_file camada 2 (A1 fix)

### V16 — view_file com credencial em arquivo → rc=2

**Payload:** `{"hookEventName":"PreToolUse","cwd":"$PROJ","toolCall":{"name":"view_file","args":{"AbsolutePath":"$TMPF"}}}`
Arquivo `$TMPF` contém `SECRET=<key>`. Rodando de project root.
**rc medido:** 2.
**Resultado:** PASS.

### V17 — view_file + arquivo benign → rc=0

**rc medido:** 0.
**Resultado:** PASS.

### V18 — A1 fix: view_file, arquivo literalmente chamado `it's.env` com chave → rc=2

**Arquivo:** `/private/tmp/hades-a1-NQ83sf/it's.env` (criado com `python3 open("it's.env","w")`; contém `SECRET=<key>`).
**Discriminador:** `its.env` (sem apóstrofe) NÃO existe no mesmo diretório — verificado com `ls`. Sem o A1 fix, `strings.Map` removeria o apóstrofe → tentaria abrir `its.env` → not found → rc=0.
**Discriminador adicional:** view_file com `AbsolutePath` inexistente → rc=0 (guard fail-open para arquivo ausente). Portanto: rc=2 para `it's.env` existente só pode resultar da A1 fix lendo o path literal → arquivo encontrado → chave detectada. Sem a fix, strips apóstrofe → tenta `its.env` → não existe → rc=0.
**rc medido:** 2 — A1 fix (`credential.go:819-836`) lê `AbsolutePath` sem remoção de aspas → arquivo com apóstrofe lido → credencial detectada.
**Resultado:** PASS — A1 fix verificado e discriminador confirmado.

---

## Surface 2 — D2 invariante

### V19 — D2: stdout vazio no allow (credential guard, Antigravity schema, sem cred)

**stdout bytes:** 0, **stderr bytes:** 0, **rc:** 0.
**Resultado:** PASS.

### V20 — D2: stdout vazio no warn (modo padrão, cred detectada)

**stdout bytes:** 0, **stderr bytes:** 60+ chars (`trackfw-credential-guard: warning...`).
**rc medido:** 0.
**Resultado:** PASS — warn não escreve no stdout. D2 invariante mantido.

---

## Surface 3 — write-scope (F5)

### V21 — write-scope: `.agents/hooks.json` absoluto, `agent_id` presente → rc=0 — FINDING F5

**Payload:** `{"tool_name":"Write","tool_input":{"file_path":"$PROJ/.agents/hooks.json","new_string":"{}"},"agent_id":"a","cwd":"$PROJ"}`
**rc medido:** 0

**Código:** `writescope.go:61-75` lista frozen paths: `.codex/hooks.json`, `.gemini/settings.json`, `.cursor/hooks.json`, `.windsurf/hooks.json`, `.github/hooks/`, `.kiro/hooks/`, `.amazonq/cli-agents/` — sem `.agents/hooks.json`.

### V22 — Contra-braço 1: `.codex/hooks.json` absoluto → rc=2

**rc medido:** 2 — `denied: ".../.codex/hooks.json" is a frozen path (.codex/hooks.json)`.
**Resultado:** PASS.

### V23 — Contra-braço 2: `trackfw.yaml` absoluto → rc=2

**rc medido:** 2 — `denied: ".../trackfw.yaml" is a frozen path (trackfw.yaml)`.
**Resultado:** PASS.

### V24 — Contra-braço 3: `.gemini/settings.json` absoluto → rc=2

**rc medido:** 2 — `denied: ".../.gemini/settings.json" is a frozen path (.gemini/settings.json)`.
**Resultado:** PASS.

---

## Surface 4 — geração de hooks.json (F2 + F3)

### V25 — F2: `enabled:false` preservado por `trackfw init` (HOME isolado)

**Setup:** `.agents/hooks.json` = `{"trackfw":{"enabled":false,"PreToolUse":[]}}` antes do `trackfw init`. HOME = `mktemp -d`.

**Medição após init:**
```
enabled: False
PreToolUse count: 4
```

**F2 CONFIRMADO.** `agentfiles.go:1983-1987` — `migrateDedupGuardInNestedArray` opera apenas no array `PreToolUse`; `enabled:false` preservado.

**Relação com ADR linha 99:** "Precedência projeto × global, `enabled:false` e timeout ainda não foram medidos. A Wave 4 mede no binário real antes do merge." A ADR adia medição do COMPORTAMENTO DO ANTIGRAVITY com `enabled:false` (M9b). F2 é a LACUNA DE IMPLEMENTAÇÃO do trackfw: `InjectAntigravityHooks` não trata `enabled:false`. Não é residual declarado.

### V26 — F3: `validate` retorna clean com `enabled:false` + 4 entradas (HOME isolado)

**Output de `validate`:**
```
✓ No violations found.
```

**F3 CONFIRMADO.** `validate` passa verde com guard desabilitado.

### V27 — Contra-braço F3: `validate` detecta ausência de entradas

**Setup:** `.agents/hooks.json` com grupo não-`trackfw`.
**Output de `validate`:**
```
⚠  .agents/hooks.json (Antigravity) is present but has no credential guard entry in event PreToolUse[run_command|manage_task]
⚠  .agents/hooks.json (Antigravity) is present but has no credential guard entry in event PreToolUse[write_to_file|...]
⚠  .agents/hooks.json (Antigravity) is present but has no credential guard entry in event PreToolUse[view_file]
```

**Resultado:** PASS — validator detecta entradas ausentes. F3 é específico ao caso `enabled:false` (entradas PRESENTES, grupo DESABILITADO).

---

## Verificações de threats T1–T12

| ID | Threat | Verificação |
|----|--------|-------------|
| T1 | exit 2 ignorado | PASS — §11 medido linha 975: `exit 2 → bloqueia` em macOS e Windows |
| T2 | Schema desconhecido → silent allow | CONFIRMADO (pré-declarado ADR); no-op para schema sem campo de comando |
| T3 | cwd != project root → no-op | PASS — A4: root-level `cwd` do payload = project root (§11 linha 998); P2 → rc=2 ✓ |
| T4 | Matcher não cobre write tools | PASS — `write_to_file\|replace_file_content\|multi_replace_file_content` em D5 |
| T5 | `enabled:false` desabilita grupo silenciosamente | F2+F3 confirmados pelo trackfw. Antigravity suprime grupo com `enabled:false` per §11 linha 295; M9b (workspace×global) não medido (§11 linha 1007, ADR linha 99) |
| T6 | Override vs merge semântico | Não medido com binário real (Wave 1 M9) |
| T7 | PATH sem trackfw → 127 | PASS — §11 linha 975: todo exit≠0 bloqueia; 127 e 9009 estão cobertos |
| T8 | Windows shell | §11 linha 982: `cmd /s /c`; formas de linha já corretas (ADR D1) |
| T-Timeout | stdin sem EOF | PASS — §11 linha 1003: `agy` fecha stdin do hook; `cat` retornou em todos os 7 braços |
| T-IDEvsAgy | Ghost install | Fora do escopo desta wave |
| T9 | BOM em hooks.json | Medido nesta sessão (HOME isolado): BOM → validate reporta 2 violations "is not valid JSON" (rc=1); PASS para o validate. Comportamento do Antigravity com hooks.json BOM'd não medido (requer agy real) |
| T10 | Home divergente no Windows | §11 linha 984: `HOME` vazio no Windows, `USERPROFILE` presente; ADR D1 usa cmd.exe forma |
| T11 | hookSpecificOutput vs decision:deny | PASS — §11: todo exit≠0 bloqueia; `hookSpecificOutput`+exit2 bloqueia (V08); T11 downgraded para LOW pós-§11 |
| T12 | decision:ask bypass | Não medido; baixa severidade per threat model |

---

## Achados por severidade

### F5 — MEDIUM (novo, confirmado com contra-braços)

**Localização:** `internal/guard/writescope.go:61-75` (`writeScopeSubagentFrozenRelPaths`)

**Descrição:** `.agents/hooks.json` ausente da frozen list enquanto `.codex/hooks.json`, `.gemini/settings.json`, `.cursor/hooks.json`, `.windsurf/hooks.json` estão presentes. Um Claude Code subagent com `agent_id` pode sobrescrever o arquivo de hooks do Antigravity com path absoluto.

**Evidência:**
- V21: `.agents/hooks.json` absoluto, `agent_id` presente → rc=0 ✗
- V22: `.codex/hooks.json` absoluto → rc=2 ✓
- V23: `trackfw.yaml` absoluto → rc=2 ✓
- V24: `.gemini/settings.json` absoluto → rc=2 ✓

**Correção:** adicionar `".agents/hooks.json"` a `writeScopeSubagentFrozenRelPaths` em `writescope.go:61-75`. Uma entrada.

---

### F2 + F3 — MEDIUM (encadeados, não declarados como residual)

**F2 — `InjectAntigravityHooks` preserva `enabled:false`:**
- Localização: `agentfiles.go:1983-1987`
- Evidência: V25 (HOME isolado) — `enabled = False` após `trackfw init`, 4 entradas adicionadas
- Fix: em `InjectAntigravityHooks`, após recuperar `trackfwGroup`, remover chave `enabled` ou emitir warning se detectado

**F3 — `validate` não detecta `enabled:false`:**
- Localização: `internal/validator/`
- Evidência: V26 (HOME isolado) — `✓ No violations found.` com guard desabilitado
- Contra-braço: V27 — validator DETECTA entradas ausentes → F3 é específico a `enabled:false`
- Fix: em `validateAntigravityGuard`, verificar campo `enabled` do grupo `trackfw`; emitir warning se `false`

**Cadeia explorável:**
1. Claude Code subagent escreve `{"trackfw":{"enabled":false,"PreToolUse":[]}}` em `.agents/hooks.json` (write-scope permite — F5).
2. `trackfw update/init` preserva `enabled:false`, adiciona 4 entradas (F2).
3. Credential guard nunca executa (grupo desabilitado pelo Antigravity — M9b confirma se workspace `enabled:false` suprime global).
4. `validate` passa verde (F3).

Sem F5 (frozen path adicionado), o passo 1 requer que o subagent já tenha escrito o arquivo por outro caminho. Com F5 fechado, a cadeia requer uma escrita anterior por outro mecanismo (write-scope de path relativo não medido nesta sessão).

---

### F4 — LOW (T11 pré-declarado, severity revista pós-§11)

**Descrição:** git-branch guard emite `hookSpecificOutput` no stdout, não `{"decision":"deny"}`.

**Evidência:** V08 — stdout = `{"hookSpecificOutput":{...}}`, rc=2.

**Status §11:** §11 medido linha 975 confirma `exit 2 → bloqueia` independentemente de stdout. `hookSpecificOutput`+exit 2 bloqueia. T11 downgraded para LOW (era ALTO no threat model pré-ML-4A, linha 612, antes dessa medição).

**Residual:** se `denyOut()` for modificado para retornar exit 0, e stdout `hookSpecificOutput` não for reconhecido como deny pelo Antigravity, fail-open. Hoje: estruturalmente mitigado.

---

### F1 — LOW (novo)

**Descrição:** `ExtractCommand` ativa P5 sem verificar `isAntigravityPayload` primeiro.

**Evidência:** V04 — rc=2 para payload sem `hookEventName` com `CommandLine = "git push"`.

**Risco:** falso positivo se runtime futuro emitir `toolCall.args.CommandLine` para outro fim.

---

## Verificações PASS (sumário)

| Check | Vetor | Resultado |
|-------|-------|-----------|
| A1 fix: `it's.env` (apóstrofe real) → rc=2; `its.env` ausente confirma discriminador | V18 | PASS |
| A4 fix: hook de `.agents/`, real payload com root-level `cwd` → rc=2 | P2 | PASS |
| D2 invariante: stdout 0 bytes em allow (credential e git-branch) | V09, V19 | PASS |
| D2 invariante: stdout 0 bytes em warn (cred detectada, modo padrão) | V20 | PASS |
| D4: lastUserInput excluído para Antigravity | C4 | PASS |
| D4 counter-arm: sem hookEventName → lastUserInput escaneado (rc=2) | C4-contra | PASS |
| D4: tool_input fora de toolCall — não detectado | P4 | PASS |
| D4: Cwd dentro de toolCall — detectado (raw scan) | P3 | PASS |
| P1>P5 (prioridade de extração) | V03 | PASS |
| P5 PascalCase case-sensitive (lowercase commandline → rc=0) | V06 | PASS |
| Global guard bloqueia Antigravity, Gemini schema e raw key | C1–C3 | PASS |
| view_file + arquivo benign → rc=0 | V17 | PASS |
| trackfw init gera 4 entradas | V25 (pré-init) | PASS |
| validate detecta ausência de guard (contra-braço F3) | V27 | PASS |
| write-scope: `.codex/hooks.json` absoluto → denied | V22 | PASS |
| write-scope: `trackfw.yaml` absoluto → denied | V23 | PASS |
| write-scope: `.gemini/settings.json` absoluto → denied | V24 | PASS |
| rc=2 estrutural em deny (git-branch, `denyOut()` direto) | V08 | PASS |
| git-branch guard funciona de `.agents/` (não depende de trackfw.yaml) | V07 | PASS |
| manage_task + cred em `Input` → rc=2 (layer 1 cobre) | P6 | PASS |
| T-Timeout: `agy` fecha stdin do hook (§11 linha 1003) | §11 medido | PASS |
| exit 2 bloqueia (§11 linha 975: macOS e Windows) | §11 medido | PASS |

---

## novos MLs na REQ-2026-11-17 (mesmo PR)

Os achados abaixo são lacunas de implementação na mesma causa (guard Antigravity): pertencem à REQ-2026-11-17 e ao mesmo PR, conforme Regra Dura de Causa Raiz.

1. **F5** (`internal/guard/writescope.go:61-75`): adicionar `".agents/hooks.json"` à frozen list.
2. **F2** (`internal/generators/agentfiles.go`, `InjectAntigravityHooks`): remover ou rejeitar `enabled:false` no grupo `trackfw` ao injetar.
3. **F3** (`internal/validator/`): checar campo `enabled` do grupo `trackfw` em `.agents/hooks.json`; emitir warning se `false`.
4. **F1** (`internal/guard/payload.go:199`): adicionar verificação `isAntigravityPayload` antes de P5, ou documentar como residual.

---

## Residual pré-existente

**manage_task `send_input` + git-branch guard:** layer 1 do credential guard detecta credencial em `Input` (P6, rc=2). Git-branch guard não bloqueia `manage_task` (matcher `run_command` não casa). Residual declarado no ADR linha 97.

**warn mode + visibilidade no TUI do `agy`:** em modo padrão (warn), guard detecta mas não bloqueia (rc=0, stdout vazio). Aviso vai para stderr; se o stderr do hook é exibido no TUI do `agy` não foi medido (Wave 1 braço adicional). Sem visibilidade, o warn é silencioso para o usuário.

**V29-B — Antigravity agent sobrescreve `.agents/hooks.json` via `write_to_file`:** agente dos runtimes (agy, IDE) pode reescrever hooks.json via tool de escrita. F5 fecha o vetor Claude Code subagents; write-scope é Claude Code only. Pré-existente; afeta todos os runtimes igualmente.

**T9 (BOM em hooks.json):** medido nesta sessão com HOME isolado. Validate detecta e reporta violations "is not valid JSON" (rc=1) — PASS para o validate. Comportamento do Antigravity ao tentar carregar um hooks.json com BOM não foi medido nesta sessão (requer agy real).

---

## Estado do repositório

```
$ git status --short
?? docs/seguranca/2026-11-18-red-team-req-2026-11-17-antigravity.md

$ git diff --cached --stat
(nenhum arquivo staged)
```

Único arquivo escrito neste ciclo: `docs/seguranca/2026-11-18-red-team-req-2026-11-17-antigravity.md`.
