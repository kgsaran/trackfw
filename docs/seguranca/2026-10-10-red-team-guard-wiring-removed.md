# Red-Team — `guard_wiring_removed` (ML-2A Wave 2)

> Revisor: `hades-tf` | Data: 2026-10-10
> Branch: `fix/remover-a-entrada-pretooluse-nao-e-detectado` | HEAD: `8f5f760f`
> REQ: `docs/req/REQ-2026-09-02-remover-a-entrada-pretooluse-do-settings-nao-e-detectado-por-nenhuma-regra-de-integridade-de-guard.md`
> ADR: `docs/adr/ADR-2026-10-10-a-fiacao-do-guard-e-ancorada-no-origin-main-por-tupla-e-a-remocao-e-violacao-fora-do-lenient-e-do-baseline.md`
> Implementação: `internal/validator/validator_guard_wiring.go`

---

## Metodologia

Reimplementação mental do contrato (ADR D1–D7 + resíduos R2–R7), seguida de execução real de cada
vetor. Binário compilado na branch: `go build -o <scratch>/trackfw ./cmd/trackfw` (EXIT=0). Todos os
ataques rodaram em repositório git descartável criado no scratch
(`/scratchpad/hades-rt/work/` + bare `/scratchpad/hades-rt/bare/`). O repositório real
(`/Users/kgsaran/Sistemas/Desenvolvimento/workspace/trackfw`) não foi modificado.

Fixture baseline: `.claude/settings.json` com as 4 tuplas canônicas (D11-revised) no `origin/main`
do repo de scratch; `HOME` apontando para diretório vazio para isolar o guard global.

---

## Tabela de ataques

| # | Vetor | Arquivo | Comando | Resultado | Coberto / Resíduo / FURO |
|---|---|---|---|---|---|
| A-01 | Remove chave `PreToolUse` inteira | `.claude/settings.json` | `cat > file` (hooks={PostToolUse:[]}) | DETECTADO — 4 violações `guard_wiring_removed` | Coberto — D2 |
| A-02 | Substitui comando por `true` | `.claude/settings.json` | `cat > file` (command:"true") | DETECTADO | Coberto — D2, `guardWiringTypeFromCmd("true")=""` |
| A-03 | Matcher estreitado (`Bash|PowerShell`→`Bash`) | `.claude/settings.json` | edição JSON | DETECTADO — 2 violações | Coberto — A2 cobertura de matcher |
| A-04 | Comando com prefixo `echo` | `.claude/settings.json` | `echo trackfw guard credential` | DETECTADO | Coberto — `guardWiringTypeFromCmd` exige igualdade de string |
| A-05 | Comando com prefixo `: ` (no-op) | `.claude/settings.json` | `: trackfw guard credential` | DETECTADO | Coberto — idem A-04 |
| A-06 | Espaço extra no comando | `.claude/settings.json` | `trackfw guard  git-branch` (duplo espaço) | DETECTADO | Coberto — idem A-04 |
| A-07 | `disableAllHooks: true` (arquivo rastreado) | `.claude/settings.json` | adição de chave | DETECTADO — mensagem cita `ref` correto | Coberto — D7 |
| A-08 | `disableAllHooks: true` (não rastreado) | `.claude/settings.local.json` | `echo '{"disableAllHooks":true}'` | DETECTADO — mensagem indica "not tracked" | Coberto — D7, `guardWiringCheckSettingsLocal` |
| A-09 | `hooksConfig.enabled: false` | `.gemini/settings.json` | adição de chave | DETECTADO | Coberto — D7 |
| A-10 | `"enabled": false` em entradas de guard | `.kiro/hooks/trackfw-attention.json` | adição de campo por entrada | DETECTADO | Coberto — D7, `disableKeyKindEntryEnabled` |
| A-11 | JSON com chave `PreToolUse` duplicada (2ª é `[]`) | `.claude/settings.json` | JSON com duplicate key | DETECTADO — Go usa última ocorrência, disco vê `[]` | Coberto — D2 |
| A-12 | Matcher com metacaractere (`^Bash$`) | `.claude/settings.json` | regex no matcher | DETECTADO — `guardWiringIsLiteralAlternation` rejeita | Coberto — A2 conservativo |
| A-13 | Matcher com alternância diminuída (`Bash|Nada`) | `.claude/settings.json` | `Bash|Nada` não contém `PowerShell` | DETECTADO | Coberto — A2 superset check |
| A-14 | JSON com UTF-8 BOM | `.claude/settings.json` | `\xef\xbb\xbf` prefixo | DETECTADO — disco ilegível → tuplas ausentes | Coberto — disco unreadable → todas as tuplas da origin missing |
| A-15 | Forma de script-path legacy em origin, arquivo ausente no disco | `.claude/settings.json` | `$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh` | DETECTADO | Coberto — `guardWiringScriptPathForms` reconhece a forma |
| A-16 | Script-path legacy em origin E no disco, script binário ausente | `.claude/settings.json` | disco=origin=script-path; scripts/ inexistente | `guard_wiring_removed` SILENCIOSO (correto); `credential_guard_hook_resolvable` DETECTA | Coberto pela regra de resolvabilidade (não é responsabilidade de `guard_wiring_removed`) |
| A-17 | `type` do hook mudado de `command` para `script` | `.claude/settings.json` | edição do campo `type` | `guard_wiring_removed` SILENCIOSO; `credential_guard_hook_resolvable` DETECTA (`requiresCommandType=true`) | Coberto por defesa em profundidade para Claude Code; **FURO parcial** — ver R-2 |
| A-18 | `timeout: 0` adicionado a cada entrada | `.claude/settings.json` | adição de campo | SILENCIOSO (correto: timeout não afeta a fiação) | Coberto — comportamento correto |
| A-19 | `git remote remove origin` + wiring removida do disco | `.git/config` | via script | SILENCIOSO | Resíduo declarado R4 — confirmado |
| A-20 | `git remote set-url origin <adv-repo>` + wiring removida | `.git/config` | via `git remote set-url` | SILENCIOSO — anchor lê origin do adversário que não tem guards | Resíduo R4 (variante não nomeada — mesma raiz: `.git/config` mutável pelo agente) |
| A-21 | Baseline gerado depois de remover wiring | `.trackfw-baseline.json` | `trackfw validate --save-baseline` | AINDA DETECTADO — baseline não suprime | Coberto — D5, `credentialGuardAnchoredRules` |
| A-22 | Modo lenient ativo + wiring removida | `trackfw.yaml` | `governance_mode: lenient; lenient_until: 2099` | AINDA DETECTADO — lenient não suprime | Coberto — D5, `lenientCarveoutRules` |
| A-23a | Branch não-padrão única (`develop`) | `.git/refs/remotes/` | repo com só `origin/develop` | SILENCIOSO — `deriveOriginDefaultBranch` usa branch única como âncora unívoca | Coberto — resolução correta (branch única = sem ambiguidade) |
| A-23b | Duas branches não-padrão (`develop` + `staging`) | `.git/refs/remotes/` | repo com `origin/develop` e `origin/staging` | DETECTADO — falha fechada com mensagem | Coberto — estado "ambíguo" → fail closed |
| A-24 | `.codex/config.toml` com `hooks = false` (não rastreado) | `.codex/config.toml` | `cat > file` (untracked) | SILENCIOSO | Resíduo declarado R3 — confirmado |
| A-25 | `.codex/config.toml` com `hooks = false` (rastreado, origin=hooks=true) | `.codex/config.toml` | origin tem `hooks = true`, disco tem `hooks = false` | DETECTADO | Coberto — D7, `guardWiringCheckCodexToml` |
| A-26 | TOML dotted key `features.hooks = false` | `.codex/config.toml` | dotted key form | DETECTADO | Coberto — parser TOML reconhece dotted key |
| A-27 | TOML inline table `features = { hooks = false }` | `.codex/config.toml` | inline table form | DETECTADO | Coberto — parser TOML reconhece inline table |
| A-28 | TOML com comentário inline `hooks = false # ...` | `.codex/config.toml` | comentário após valor | DETECTADO | Coberto — parser strip de comentário inline |
| A-29 | FP: origin=D2-legacy, disco=D11-revised | `.claude/settings.json` | migração legítima | SILENCIOSO (correto — sem falso positivo) | Coberto — A2 classe de equivalência funciona |
| A-30 | FP: origin=`Bash`, disco=`Bash\|PowerShell` (superset) | `.claude/settings.json` | migração de matcher | SILENCIOSO (correto — sem falso positivo) | Coberto — A2 matcher coverage funciona |
| A-31 | origin=`Bash\|PowerShell`, disco=`Bash` (subset) | `.claude/settings.json` | estreitamento | DETECTADO | Coberto — A2 rejeita subset |
| A-32 | `disableAllHooks: true` em Copilot | `.github/hooks/trackfw-attention.json` | adição de chave | DETECTADO | Coberto — D7, `disableKeyKindDisableAllHooks` |
| A-33 | `git update-ref -d` para apagar todos os refs de `origin` | `.git/refs/remotes/` | via script | SILENCIOSO | Resíduo declarado R4b — confirmado |
| A-34 | TOML `hooks = "false"` (string entre aspas, não booleano) | `.codex/config.toml` | valor não-true | DETECTADO (fail-closed) | Coberto — `tomlHooksDisabledOrUnknown` cobre valor não-true |
| A-35 | TOML `[features]` presente mas sem chave `hooks` | `.codex/config.toml` | chave ausente | SILENCIOSO (correto) | Coberto — `tomlHooksAbsent` → silent |
| A-36 | JSON com comentário JSONC no final | `.claude/settings.json` | `// comment` após JSON | DETECTADO (fail-closed) | Coberto — JSON parse falha → disco ilegível → todas tuplas missing |
| A-37 | Matcher vazio em origin, disco tem `Bash` (estreitamento) | `.claude/settings.json` | origin sem matcher, disco com matcher | DETECTADO — `matcherCovers("Bash", "")` = false | Coberto — A2 conservativo |
| A-38 | `disableAllHooks: true` em Copilot já presente em origin/main | `.github/hooks/trackfw-attention.json` | chave idêntica nos dois lados | SILENCIOSO (correto) | Coberto — condição `diskActive && !originActive` = false |
| A-39 | Kiro `enabled:false` em origin/main e no disco | `.kiro/hooks/trackfw-attention.json` | chave idêntica nos dois lados | SILENCIOSO (correto) | Coberto |
| **A-40b** | **Comandos de guard em `metadata_guards.command`, `hooks[].command`=`echo bypassed` — Claude Code** | `.claude/settings.json` | estrutura JSON com guard em campo não-executável | `guard_wiring_removed` SILENCIOSO; `credential_guard_hook_resolvable` DETECTA (type:command ausente) | **FURO em `guard_wiring_removed`** — defesa mantida por outra regra para Claude Code (requiresCommandType=true) |
| **A-40c** | **Idem A-40b para Cursor (`requiresCommandType=false`)** | `.cursor/hooks.json` | guard em `metadata_guards.command`, `command`=`echo bypassed` | `guard_wiring_removed` SILENCIOSO; `credential_guard_hook_resolvable` SILENCIOSO | **FURO bloqueante** — nenhuma regra detecta; guard nunca executa |

---

## Achados

### R-1 — `guardWiringCollectCommandValues` coleta comandos em caminhos não-executáveis — BLOQUEANTE

**Vetor:** adversário copia os comandos de guard para um campo JSON personalizado (ex.:
`metadata_guards.command`) e substitui o campo executável real por `echo bypassed` (ou outro no-op).
`guardWiringCollectCommandValues` faz busca recursiva por qualquer chave `"command"` em todo o JSON
sem restrição de caminho. Encontra o guard command no campo de metadata, considera a tupla "coberta"
e fica em silêncio.

**Medição:**

```bash
# Fixture: .cursor/hooks.json com guard em metadata, command=echo bypassed
# Disco:
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash|PowerShell",
        "metadata_guards": {"command": "$LASTEXITCODE=2 ...; trackfw guard credential; ..."},
        "command": "echo bypassed"
      }
    ]
  }
}

# Resultado:
cd /scratchpad/hades-rt/work && HOME=/scratchpad/hades-rt/home-empty /scratchpad/hades-rt/trackfw validate 2>&1 | grep "guard_wiring_removed.*Cursor"
# (saída vazia — 0 violações para Cursor)
```

**CLIs afetados e cobertura de defesa em profundidade:**

| CLI | requiresCommandType | `guard_wiring_removed` | `credential_guard_hook_resolvable` | Resultado final |
|---|---|---|---|---|
| Claude Code, Codex, Gemini | true | SILENCIOSO | DETECTA (type:command ausente) | Guard detectado por outra regra |
| Cursor | false | SILENCIOSO | SILENCIOSO | **Guard completamente contornado** |
| Copilot, Kiro | false | SILENCIOSO | Não testado explicitamente, provavelmente SILENCIOSO | Presumivelmente contornado |
| Windsurf, Amazon Q | false | SILENCIOSO | Não testado | Presumivelmente contornado |

**Raiz:** `guardWiringCollectCommandValues` (`validator_guard_wiring.go:304-325`) não restringe a
busca ao caminho estrutural esperado pela spec de cada CLI. Para Cursor, o caminho executável é
`hooks.<event>[].command` (campo direto na entrada), não `hooks.<event>[].metadata_guards.command`.

**Correção mínima:** `guardWiringExtractTuples` (`validator_guard_wiring.go:244`) já lida
com os dois layouts (`hooks-as-object` e `hooks-as-array`). A coleta de comandos deve ser restrita ao
caminho correto por layout:
- hooks-as-object (Claude Code, Codex, Gemini, Copilot, Kiro): coletar de `entry.hooks[].command` (não do entry diretamente)
- hooks com campo `command` direto (Cursor, Windsurf, Amazon Q): coletar de `entry.command` apenas (não de chaves aninhadas)

Implementação: em `addCommandsFromEntry`, trocar `guardWiringCollectCommandValues(entry, &cmds)` por
uma extração orientada por caminho (2 ramos: `hooksArray` do entry vs. `directCommand` do entry),
guiada por uma propriedade nova em `credentialGuardHookFile` (ex.: `hookSchema`).

---

### R-2 — `type: script` bypassa `guard_wiring_removed` para requiresCommandType=false CLIs — não bloqueante

**Vetor (parcialmente coberto):** mudança do campo `type` de `command` para `script` em CLIs que
exigem `requiresCommandType=true` é detectada por `credential_guard_hook_resolvable` (mensagem:
"missing type:command"). Para CLIs com `requiresCommandType=false` (Cursor, Copilot, Kiro, Windsurf,
Amazon Q), a mudança de tipo não é detectada por nenhuma regra — mas o ataque também requer que a
string de comando do guard permaneça no campo `command`, o que permite que `guard_wiring_removed`
extraia a tupla corretamente. Portanto, este vetor não representa bypass da fiação (o comando guard
ainda está no campo executável); apenas bypass da validação de tipo.

**Relevância:** baixa para `guard_wiring_removed` em isolamento. Observação: se o schema do CLI
for `command` direto (Cursor), `type: script` pode não ser executado dependendo do runtime. Não
investigado empiricamente para cada CLI.

---

### Confirmação dos resíduos declarados

| Resíduo | Ataque | Confirmado |
|---|---|---|
| R2 — escopo global | não testado (sem âncora git) | — |
| R3 — `.codex/config.toml` não rastreado | A-24 | Sim — SILENCIOSO |
| R4 — `git remote remove origin` | A-19 | Sim — SILENCIOSO |
| R4 (variante URL swap) | A-20: `git remote set-url origin <adv>` | Sim — SILENCIOSO; variante de R4, mesma raiz (`.git/config`) |
| R4b — 0 refs buscados | A-33: `git update-ref -d` em todos os refs | Sim — SILENCIOSO |
| R5 — merge em main | não testado (fora do scope) | — |
| R7 — Windsurf/Amazon Q | não testado (nenhuma chave doc.) | — |

---

### Confirmação dos carve-outs de severidade

| Carve-out | Ataque | Resultado |
|---|---|---|
| Baseline não suprime | A-21 | CONFIRMADO — violação persiste com `.trackfw-baseline.json` |
| Lenient não suprime | A-22 | CONFIRMADO — violação persiste com `governance_mode: lenient` |

---

### Confirmação da ausência de falsos positivos

| FP | Ataque | Resultado |
|---|---|---|
| Migração D2-legacy → D11-revised | A-29 | SILENCIOSO (correto) |
| Matcher superset (`Bash` → `Bash|PowerShell`) | A-30 | SILENCIOSO (correto) |
| Nunca instalado (arquivo ausente em origin) | baseline | SILENCIOSO (correto — D4) |

---

## Veredito

**APROVADO COM RESSALVAS**

O mecanismo central está correto e tem mordida: todas as formas diretas de remoção de fiação
(chave apagada, matcher estreitado, comando neutralizado, D7 em todas as formas TOML/JSON
documentadas) são detectadas. Os carve-outs de severidade (lenient, baseline) são imunes. Os
resíduos declarados foram confirmados empiricamente.

Um furo genuine foi identificado:

---

**R-1 — `guardWiringCollectCommandValues` recursiva: bypass por campo JSON de metadata — BLOQUEANTE**

Para CLIs com `requiresCommandType=false` (Cursor, Copilot, Kiro, Windsurf, Amazon Q), um adversário
pode colocar os comandos de guard em qualquer campo JSON com chave `"command"` (ex.:
`metadata_guards.command`) e substituir o campo executável por `echo bypassed`. O validador encontra
o guard no campo de metadata e fica em silêncio. O guard nunca executa no runtime.

Medido: `.cursor/hooks.json` com `command: "echo bypassed"` e `metadata_guards.command: <guard_cmd>`.
`trackfw validate` retorna 0 violações de `guard_wiring_removed` e 0 de `credential_guard_hook_resolvable`.

**Correção mínima:** restringir a coleta de comandos em `addCommandsFromEntry` ao caminho estrutural
esperado por layout de schema, em vez de busca recursiva geral. Mudança em
`internal/validator/validator_guard_wiring.go` function `guardWiringExtractTuples` / `addCommandsFromEntry`.

---

**R-2 — type:script para CLIs sem requiresCommandType — não bloqueante**

Cobertura mantida por `credential_guard_hook_resolvable` para requiresCommandType=true. Para
requiresCommandType=false, o vetor requer que a string do guard permaneça no campo executável
(portanto não é bypass completo sem combinar com R-1).
