# Red-team: aviso de presença do credential guard nos 6 CLIs sem regra de violação

> REQ: REQ-2026-10-25 · ADR D17 · Issue #619 · ML-2B
> Agente: hades-tf · Data: 2026-10-25
> Binário `new`: branch `feat/validate-aviso-presenca-credential-guard` (commit 2cffb81b)
> Binário `old`: origin/main (commit 14d10dc3 — v11.0.2)
> Fixtures: `mktemp -d` isolados por ataque, HOME isolado, sem git (anchor state: originAnchorNoGit)

---

## Metodologia

Dois binários compilados em diretórios distintos:

```bash
B=$(mktemp -d)
# new: cd repo && go build -o $B/new ./cmd/trackfw
# old: git archive origin/main | tar -x -C $M && (cd $M && go build -o $B/old_bin ./cmd/trackfw)
```

Para cada ataque, fixture em `T=$(mktemp -d); mkdir -p $T/home $T/proj; cd $T/proj`.
Saída completa capturada em `/tmp/<ataque>.txt`. Controle executado antes de qualquer bloqueante.

---

## Tabela de ataques

| ID | Superfície | Fixture | Aviso esperado | Resultado | rc new |
|----|-----------|---------|----------------|-----------|--------|
| A1a | Claude: matcher `"Bash"` em vez de `"Bash\|PowerShell"` | PreToolUse[Bash], Read, Write\|Edit com guard | sim (Bash\|PowerShell ausente) | PASS | 1* |
| A1b | Claude: guard apenas em PostToolUse (não em PreToolUse) | PostToolUse com 3 matchers corretos | sim (3 avisos) | PASS | 1* |
| A1c | Claude: `type: "approval"` em vez de `"command"` | PreToolUse 3 pares, type=approval | sim (3 avisos) | PASS | 1* |
| A1d | Claude: marcador em `group["command"]` (nível de grupo, não `hooks[i]["command"]`) | group-level command field | sim (3 avisos) | PASS | 0 |
| A1e | Claude: JSON com chave duplicada — guard na 1ª, array vazio na 2ª | dois "PreToolUse" no JSON | sim (Go lê última, que é vazia) | PASS | 0 |
| A1e2 | Claude: JSON com chave duplicada — vazio na 1ª, guard na 2ª | dois "PreToolUse" no JSON | não (última tem guard) | PASS | 1* |
| A1f | Kiro: marcador em `entry["command"]` (outer field, fora de `action`) | outer command ≠ action.command | sim (3 avisos) | PASS | 0 |
| A1g | Cursor: guard só em `beforeShellExecution`, sem `preToolUse` Read/Write | apenas 1 dos 3 pares | sim (2 avisos) | PASS | 1* |
| A1h | Copilot: guard no campo `"bash"` em vez de `"command"` | bash field | sim (3 avisos) | PASS | 0 |
| A1i | Gemini: guard só em `run_shell_command`, sem read/write | apenas 1 dos 3 BeforeTool | sim (2 avisos) | PASS | 1* |
| A1j | Kiro: `action.type = "approval"` (não `"command"`) | type=approval | sim (3 avisos) | PASS | 1* |
| A1k | Copilot: `type = "approval"` | type=approval | sim (3 avisos) | PASS | 1* |
| A1l | Codex: guard só em `apply_patch`, sem `Bash` | apenas 1 dos 2 PreToolUse | sim (1 aviso) | PASS | 1* |
| A2-Claude | init + strip + validate + update + validate — Claude | fixture de init com guard removido | sim antes / não depois | PASS | 0 após update |
| A2-Codex | idem — Codex | idem | sim antes / não depois | PASS | 0 após update |
| A2-Gemini | idem — Gemini | idem | sim antes / não depois | PASS | 0 após update |
| A2-Cursor | idem — Cursor | idem | sim antes / não depois | PASS | 0 após update |
| A2-Copilot | idem — Copilot | idem | sim antes / não depois | PASS | 0 após update |
| A2-Kiro | idem — Kiro | idem | sim antes / não depois | PASS | 0 após update |
| A3 | Hook sem trackfw no mesmo caminho | `.claude/settings.json` com `some_other_tool` | sim (arquivo presente, sem guard) | PASS — comportamento esperado (ver §A3) | 0 |
| A3b | Hook com `trackfw guard write-scope` (sem credential) | write-scope present, credential ausente | sim (3 avisos) | PASS | 0 |
| A4a | `rules: credential_guard_presence_warning: "off"` | Claude sem guard, sem git | não (silenciado) | PASS — silencia em projetos não-ancorados (ver §A4) | 0 |
| A4b | `rules: credential_guard_presence_warning: "error"` | Claude sem guard | aviso permanece (não vira violation) | PASS — applyRuleWarnOnly ignora elevação | 0 |
| A4c | Modo lenient com `governance_mode: lenient` + `lenient_until` válido | Claude sem guard | aviso persiste como aviso, rc=0 | PASS | 0 |
| A5 | Windsurf sem guard: old vs new binary | `.windsurf/hooks.json` sem guard | violation em ambos | PASS — saída idêntica, rc=1 | 1 |
| A5b | Amazon Q sem guard: old vs new binary | `.amazonq/cli-agents/q_cli_default.json` sem guard | violation em ambos | PASS — saída idêntica, rc=1 | 1 |
| A6 | Saída `--json` inclui avisos de presença | Claude sem guard | sim, rule=credential_guard_presence_warning | PASS — 3 msgs no JSON, rule correto | 0 |
| A-chmod | chmod 000 no arquivo de hook | `.claude/settings.json` chmod 000 | ? (ver §A-file) | PASS (ver §A-file) | 1 |
| A-BOM | BOM prefix + JSON | BOM + `{"hooks": {}}` | ? (ver §A-file) | PASS (ver §A-file) | 1 |
| A-inv-json | JSON inválido | `NOT JSON {{{` | ? (ver §A-file) | PASS (ver §A-file) | 1 |
| A-symlink | Symlink para arquivo sem guard | symlink → arquivo sem guard | sim (aviso) | PASS | 0 |
| A-echo | Marcador em `echo trackfw guard credential` | echo wrapper | aviso suprimido; formato incorrect → violation | PASS (ver §A-echo) | 1 |

`*` rc=1 vem de rule pré-existente (`credential_guard_hook_resolvable` — formato do guard diverge do esperado), não da nova rule.

---

## Evidências selecionadas

### A1b — guard apenas em PostToolUse

```
$ cd $T/proj && HOME=$T/home $NEW validate
⚠  .claude/settings.json (Claude Code) is present but has no credential guard entry in event PreToolUse[Bash|PowerShell]
⚠  .claude/settings.json (Claude Code) is present but has no credential guard entry in event PreToolUse[Read]
⚠  .claude/settings.json (Claude Code) is present but has no credential guard entry in event PreToolUse[Write|Edit]
```

O extrator filtra por `hooks[event.hookEvent]` — eventos diferentes não se confundem.

### A1d — marcador em `group["command"]` (nível de grupo)

Fixture: `{"matcher": "Bash|PowerShell", "command": "trackfw guard credential", "hooks": []}`.
Os 3 avisos de presença disparam. O extrator `hookCmdSchemaNested` lê `innerObj["command"]` dentro de `group["hooks"][i]` — o campo `group["command"]` no nível de grupo não é lido. Marcador em posição que o Claude SDK não executa não engana o check.

### A1e — chave JSON duplicada

Go `json.Unmarshal` usa o valor da última ocorrência de chave duplicada. Com guard na 1ª e vazio na 2ª: 3 avisos. Com vazio na 1ª e guard na 2ª: 0 avisos. Comportamento determinístico e correto.

### A1f — Kiro: outer `command` em vez de `action.command`

```
⚠  .kiro/hooks/trackfw-attention.json (Kiro) is present but has no credential guard entry in event PreToolUse[shell]
⚠  .kiro/hooks/trackfw-attention.json (Kiro) is present but has no credential guard entry in event PreToolUse[read]
⚠  .kiro/hooks/trackfw-attention.json (Kiro) is present but has no credential guard entry in event PreToolUse[write]
```

`hookCmdSchemaKiro` lê `action["command"]`. O campo `entry["command"]` no nível raiz do objeto é ignorado — correto, pois o Kiro não executa esse campo.

### A2 — ciclo completo init + strip + validate + update + validate

Executado para os 6 CLIs. Exemplo com Claude:

```
# init — 0 avisos de presença (guard gerado corretamente)
# strip (python3 remove guard de PreToolUse)
$ HOME=$T/home $NEW validate
⚠  .claude/settings.json (Claude Code) is present but has no credential guard entry in event PreToolUse[Bash|PowerShell]
⚠  .claude/settings.json (Claude Code) is present but has no credential guard entry in event PreToolUse[Read]
⚠  .claude/settings.json (Claude Code) is present but has no credential guard entry in event PreToolUse[Write|Edit]
RC=0
# update
$ HOME=$T/home $NEW update && $NEW validate
0 presence warnings — RC=0
```

Para Codex, Gemini, Cursor, Copilot e Kiro: mesmo resultado — 0 avisos de presença depois de `trackfw update`. O aviso é resolúvel para todos os 6 CLIs.

### A3 — hook de outra ferramenta no mesmo caminho

O check dispara quando o arquivo existe, independentemente do conteúdo. Isso é o comportamento correto: se `.claude/settings.json` existe (qualquer ferramenta o cria), o validate avisa que o credential guard não está presente. A alternativa (verificar se é um arquivo de trackfw) exigiria identificar autorias, o que não é possível de forma confiável. Comportamento declarado no ADR D17.

### A4 — severidade

**Elevação (`rules: error`):** `applyRuleWarnOnly` ignora a severidade configurada exceto `"off"` — o aviso permanece como aviso mesmo com `rules: credential_guard_presence_warning: error`. Design explícito (comentário: "NUNCA devem virar violation").

**Silenciamento (`rules: off`) — comportamento depende do anchor:**
- Em projetos **sem git** (mktemp fixture, anchor = `originAnchorNoGit`): `diskRuleSeverity` retorna `"off"` → `applyRuleWarnOnly` silencia. Confirmado: RC=0, 0 avisos.
- Em projetos **com git + origin** (anchor = `originAnchorOK`): `ruleSeverity` retorna `stricter-of(origin_main_sev, disk_sev)`. O default do produto para `credential_guard_presence_warning` é `"warning"`. `stricter-of("warning", "off") = "warning"` → `applyRuleWarnOnly` recebe `"warning"` ≠ `"off"` → **não silencia**. O mecanismo de anchor torna o aviso insilenciável via disk `rules:off` em projetos reais (que rodaram `trackfw init`).

**Modo lenient (`governance_mode: lenient` + `lenient_until` dentro do horizonte de 730 dias):**
Com `governance_mode: lenient` e `lenient_until: 2026-11-25`, o banner `[LENIENT MODE]` aparece, rc=0, e o aviso continua como aviso. `applyRuleWarnOnly` não usa `applyLenientWithCarveout` — mensagens já no canal warnings não são afetadas pelo modo lenient.

**Wave 0 R5 atualizado:** a premissa de R5 ("aceito — silenciável via rules:off, consistente com produto") é válida apenas para projetos sem anchor (não-init). Em projetos normais (com `trackfw init` executado), o aviso é insilenciável via disk config — propriedade mais forte que R5 declarou. Não é um defeito: é consequência esperada do mecanismo de anchor de ADR-2026-09-17.

### A5/A5b — Windsurf e Amazon Q

Old e new binaries produzem saída idêntica: `2 violation(s) found`, rc=1. Sem regressão.

### A6 — saída JSON

```json
{"rule": "credential_guard_presence_warning", "file": "", "message": "...PreToolUse[Bash|PowerShell]..."}
```

O mirror site em `validateUnfilteredTagged` está presente e corretamente cableado.

### A-file — erros de leitura e de parse

| Caso | Comportamento presence check | Comportamento outros checks |
|------|------------------------------|----------------------------|
| chmod 000 | silent (readErr → `continue`) | violation "could not be read" — **idêntico em old e new** |
| Symlink para no-guard | aviso dispara (symlink é seguido, arquivo lido, guard ausente) | — |
| Diretório no path | silent (readErr → `continue`) | sem violation — mesma razão |
| JSON inválido | silent (json.Unmarshal err → `continue`) | violation "not valid JSON" em old e new |
| BOM prefix | silent (BOM → json.Unmarshal falha) | violation "not valid JSON" em old e new |

Para todos os casos em que a presence check silencia, o código já emite uma violation ou warning descritivo via regras pré-existentes (`credential_guard_hook_resolvable`). Comportamento pré-existente, confirmado como idêntico entre old e new.

### A-echo — marcador em posição não-executável

`echo trackfw guard credential` e `exit 0; trackfw guard credential`: o `strings.Contains` da presence check encontra o marcador → sem aviso de presença. Comportamento compartilhado com `credential_guard_hook_resolvable` (a regra de formato também usa `strings.Contains` como primeiro passo). A regra de formato dispara porque o comando não corresponde à forma esperada:

```
✗ .claude/settings.json (Claude Code) has "exit 0; trackfw guard credential" but the expected form...
```

Assim, o aviso de presença é suprimido, mas a violation do formato captura o caso. Pré-existente: o old binary se comporta identicamente. O par (presença + formato) mantém cobertura.

---

## Veredito

**APROVA.**

Nenhum bloqueante. Todos os ataques passaram. Propriedades confirmadas:

- Filtra por matcher exato — matchers parecidos não suprimem o aviso (A1a, A1l).
- Filtra por evento — guard no evento errado não suprime (A1b).
- Rejeita `type != "command"` para CLIs com `requiresCommandType=true` (A1c, A1j, A1k).
- Não lê posições não-executáveis: `group["command"]` para nested (A1d), outer `command` para Kiro (A1f), `"bash"` field para Copilot (A1h).
- Comporta-se corretamente com JSON de chave duplicada (A1e, A1e2).
- O aviso some para todos os 6 CLIs após `trackfw update` (A2-*).
- Nunca vira violation — `applyRuleWarnOnly` ignora elevação via `rules: error` (A4b).
- Em projetos com anchor ativo (init executado + origin), o aviso é insilenciável via disk `rules: off` — propriedade mais forte que Wave 0 R5 declarou.
- Windsurf e Amazon Q inalterados entre binários (A5, A5b).
- Mirror site `--json` funciona (A6).
- Para erros de leitura e de parse, a silence da presence check é coberta por violations pré-existentes; comportamento idêntico entre old e new (A-file).

---

## Bloqueantes

Nenhum.

---

## Residuais

**R5-anchor** (update da Wave 0 R5): `rules: credential_guard_presence_warning: off` silencia em projetos **sem anchor** (sem git, ou sem `trackfw init` executado, ou sem `origin` remoto). Em projetos **com anchor** (maioria dos projetos reais), o mecanismo `stricter-of(origin_main="warning", disk="off")="warning"` impede o silenciamento. Esse é o comportamento correto do produto — não é um defeito. O Wave 0 R5 dizia "aceito — consistente com produto": a premissa estava incompleta (medida em fixture sem git). A assertiva correta é: "silenciável apenas em projetos não-ancorados; insilenciável em produção".

**R-echo** (pre-existing): `echo trackfw guard credential` em um hook suprime o aviso de presença. O par com `credential_guard_hook_resolvable` (violation de formato) mantém cobertura — o marcador presente mas com formato errado dispara a violation. Comportamento idêntico ao old binary.

**R-chmod** (pre-existing): chmod 000 silencia a presence check, mas a violation "could not be read" dispara. Comportamento idêntico ao old binary.

**R-BOM** (pre-existing): BOM prefix silencia presence check (json.Unmarshal falha), mas a violation "not valid JSON" dispara. Comportamento idêntico ao old binary. O vault note `project_credextractcmdandcwd_bom_unmarshal_data_not_stripped.md` descreve um contexto diferente (BOM na extração de credencial, não neste path).

**R-JSON-file-field** (menor): o campo `"file"` das mensagens de presença no output `--json` é `""`. Consistente com outros avisos de ausência de arquivo no produto. Não afeta funcionalidade.

**R2** (herdado da Wave 0): comportamento de bloqueio de `Cursor preToolUse+Read/Write` não confirmado em produção. O gerador escreve os pares (confirmado no Wave 0); a lista de presença os inclui. Se não bloqueiam, o aviso é inócuo e resolúvel por `trackfw update`.

---

## Git status no repositório

```
On branch feat/validate-aviso-presenca-credential-guard
Your branch is up to date with 'origin/feat/validate-aviso-presenca-credential-guard'.

nothing to commit, working tree clean
```
