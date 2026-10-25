# Wave 0 — Marcador do guard fora de hook: threat model e matriz de sítios

**Roadmap:** ROADMAP-2026-10-25-validate-casa-o-marcador-do-guard-em-qualquer-string-do-settings-e-nao-so-em-comando-de-hook.md
**ML:** ML-0A
**Data:** 2026-10-25
**Agente:** hades-tf
**Binário medido:** trackfw 11.0.1

---

## 1 — Completude da enumeração

### 1.1 Superfícies declaradas no roadmap

O roadmap descreve uma superfície única: `collectCommandsWithMarker`
(`internal/validator/validator_credential_guard.go:332`) percorre o JSON inteiro e casa o marcador
em qualquer string, afetando 12 chamadores que passam o arquivo completo.

**A enumeração está incompleta em dois pontos:**

**a) Segundo sítio de busca por substring em `scripts/`**

`scripts/check-gates-falsify.sh` usa `grep -qF` em caminhos gerados pelo próprio harness (não em
arquivos de usuário), nas linhas:

| Linha | Padrão buscado | Propósito |
|---|---|---|
| 5566, 5577 | `grep -qF 'trackfw guard credential;'` | verifica presença no settings.json gerado |
| 5606, 5677 | `grep -qF 'trackfw guard git-branch'` | verifica presença no settings.json gerado |
| 5835–5836, 5887–5888 | mesmos padrões | verifica presença nos arquivos de harness gerados |

Esses `grep` operam sobre o **output do gerador** (`trackfw init`/`trackfw update`), não sobre
arquivos de usuário. O gerador escreve o marcador sempre no campo de comando correto; portanto esses
grep sempre passam e nunca produzem falso positivo. Eles **não são afetados pelo defeito** e **não
precisam mudar**, mas devem ser mencionados para fechar a enumeração.

`scripts/windows-repro/guard-global-home.ps1:62–64` usa `-match "trackfw guard"` e `-match
"trackfw-credential-guard\.sh"` sobre conteúdo bruto do arquivo para fins de reprodução. Script de
diagnóstico, não de validação — fora de escopo.

**b) Copilot — assimetria de chave entre projeto e global**

O arquivo de projeto Copilot (`.github/hooks/trackfw-attention.json`) usa a chave `"command"` para
o comando de hook. O arquivo global Copilot (`~/.copilot/settings.json`) usa a chave **`"bash"`**
para o mesmo propósito. `collectCommandsWithMarker` casa em qualquer string hoje, então as duas
formas são encontradas. Após a correção — que restringirá a busca a campos de comando por esquema —
a coleta para Copilot global **deve ler `"bash"`, não `"command"`**. Se a implementação restringir
apenas a `"command"`, o harness global Copilot será invisível ao validador (fail-open silencioso
para um caso que hoje funciona corretamente).

**c) Windsurf global ausente de `globalGuardConfigFiles`**

`~/.codeium/windsurf/hooks.json` **não está em `globalGuardConfigFiles`**
(`validator_git_branch_guard.go:91`). O validador nunca lê esse arquivo. Esse gap existe antes
deste defeito e persiste após a correção — deve ser registrado como residual declarado.

### 1.2 Campos de comando por CLI — projeto e global

A tabela abaixo foi derivada da leitura de `credentialGuardHookFiles` (L72 de
`validator_credential_guard.go`) e `globalGuardConfigFiles` (L91 de
`validator_git_branch_guard.go`), cruzada com o código do gerador
(`internal/generators/agentfiles.go`).

| CLI | Escopo | Arquivo | Caminho JSON do comando | Chave de comando | Requer `type:"command"` |
|---|---|---|---|---|---|
| Claude Code | projeto | `.claude/settings.json` | `hooks.PreToolUse[].hooks[].command` | `command` | sim |
| Claude Code | global | `<claudeDir>/settings.json` | `hooks.PreToolUse[].hooks[].command` | `command` | sim |
| Codex CLI | projeto | `.codex/hooks.json` | `hooks.PreToolUse[].hooks[].command` | `command` | sim |
| Codex CLI | global | `~/.codex/hooks.json` | `hooks.PreToolUse[].hooks[].command` | `command` | sim |
| Gemini CLI | projeto | `.gemini/settings.json` | `hooks.BeforeTool[].hooks[].command` | `command` | sim |
| Gemini CLI | global | `~/.gemini/settings.json` | `hooks.BeforeTool[].hooks[].command` | `command` | sim |
| Cursor | projeto | `.cursor/hooks.json` | `hooks.beforeShellExecution[].command` | `command` | não (array plano) |
| Cursor | global | `~/.cursor/hooks.json` | `hooks.beforeShellExecution[].command` | `command` | não (array plano) |
| GitHub Copilot CLI | projeto | `.github/hooks/trackfw-attention.json` | `hooks.preToolUse[].command` | `command` | sim (`type:"command"`) |
| GitHub Copilot CLI | global | `~/.copilot/settings.json` | `hooks.preToolUse[].bash` | **`bash`** | sim — **ASSIMÉTRICO** |
| Kiro | projeto | `.kiro/hooks/trackfw-attention.json` | `hooks[].action.command` | `command` | sim (`action.type:"command"`) |
| Kiro | global (credential) | `~/.kiro/hooks/trackfw-credential-guard.json` | `hooks[].action.command` | `command` | sim |
| Kiro | global (git-branch) | `~/.kiro/hooks/trackfw-git-branch-guard.json` | `hooks[].action.command` | `command` | sim |
| Windsurf | projeto | `.windsurf/hooks.json` | `hooks.pre_run_command[].command` | `command` | não (array plano) |
| Windsurf | global | `~/.codeium/windsurf/hooks.json` | `hooks.pre_run_command[].command` | `command` | não — **AUSENTE de globalGuardConfigFiles** |
| Amazon Q | projeto | `.amazonq/cli-agents/q_cli_default.json` | `hooks.preToolUse[matcher].hooks[].command` | `command` | não (array plano) |

### 1.3 Matriz de chamadores de `collectCommandsWithMarker`

Levantamento dos 14 chamadores:

| Arquivo | Linha | Argumento passado | Regra alimentada | Recebe arquivo inteiro? |
|---|---|---|---|---|
| `validator_credential_guard.go` | L512 | `parsed` (arquivo inteiro) | `_hook_resolvable` (violation) | **SIM** |
| `validator_credential_guard.go` | L583 | `parsed` (arquivo inteiro) | forma da linha (violation) | **SIM** |
| `validator_credential_guard.go` | L682 | `parsed` (arquivo inteiro) | presença (fail-open se conta prosa) | **SIM** |
| `validator_credential_guard.go` | L758 | `hooks[event.hookEvent]` (subtree de evento) | presença via `extractCredentialGuardCommandsFromEvent` — seguro para prosa fora de `hooks`; ainda afetado para campo não-command dentro de hook (AC4) | parcial |
| `validator_credential_guard.go` | L773 | `obj["hooks"]` (subtree de matcher Amazon Q) | presença via `extractCredentialGuardCommandsFromEvent` — seguro para prosa fora de `hooks`; ainda afetado para campo não-command dentro de hook (AC4) | parcial |
| `validator_credential_guard.go` | L888 | `parsed` (arquivo inteiro) | forma da linha (violation) | **SIM** |
| `validator_credential_guard.go` | L942 | `parsed` (arquivo inteiro) | forma da linha (violation) | **SIM** |
| `validator_credential_guard.go` | L995 | `parsed` (arquivo inteiro) | forma da linha (violation) | **SIM** |
| `validator_git_branch_guard.go` | L259 | `parsed` (arquivo inteiro) | presença (fail-open se conta prosa) | **SIM** |
| `validator_git_branch_guard.go` | L313 | `parsed` (arquivo inteiro) | forma da linha (violation) | **SIM** |
| `validator_git_branch_guard.go` | L424 | `parsed` (arquivo inteiro) | forma da linha (violation) | **SIM** |
| `validator_git_branch_guard.go` | L482 | `parsed` (arquivo inteiro) | presença (fail-open se conta prosa) | **SIM** |
| `validator_git_branch_guard.go` | L549 | `parsed` (arquivo inteiro) | forma da linha (violation) | **SIM** |
| `validator_git_branch_guard.go` | L620 | `parsed` (arquivo inteiro) | presença (fail-open se conta prosa) | **SIM** |

**Dois sítios parcialmente seguros:** L758 e L773, chamados via `extractCredentialGuardCommandsFromEvent`,
passam apenas o array do evento (`hooks[event.hookEvent]` ou `obj["hooks"]`). Isso os torna seguros
para o falso positivo do tipo #618 (prosa em `autoMode.environment`, que está fora de `hooks`), mas
**não** para o fail-open de AC4: `collectCommandsWithMarker` ainda percorre todos os campos de string
dos objetos dentro desse array, incluindo `"note"`. Todo chamador — inclusive L758 e L773 — requer
uma versão schema-aware do coletor que só leia o campo de comando correto por CLI.

**Sítios fora do escopo (não usam `collectCommandsWithMarker`):**
- `validator_write_scope_wiring.go` — função `settingsHasWriteScopeHook()` navega o esquema
  diretamente: `root["hooks"]["PreToolUse"][].matcher == "Write|Edit|NotebookEdit"` →
  `entry["hooks"][].command`. Schema-aware. Não afetado.
- `validateClaudeGuardHookMatcherWarningsInFile` (`validator_credential_guard.go:L1072`) — restringe
  a `hObj["command"].(string)` dentro de `innerHooks`. Schema-aware. Não afetado.

---

## 2 — Modelo de ameaça

O adversário neste Wave 0 não é um atacante externo: é o **implementador apressado** que corrige o
defeito — e o **arquiteto otimista** que audita sem medir os dois braços do contrato.

### 2.1 Ameaça A — Fix incompleto: restringe a chave mas não o nível do objeto

O implementador lê "restringir a campo de comando" como "só processar entradas em que `key ==
"command"`". O novo coletor percorre o JSON procurando `"command"` em qualquer objeto, sem exigir
que o objeto-pai seja um elemento de array de hook no nível correto do esquema.

**Superfície de entrada:** objeto JSON aninhado com `"command": "trackfw guard credential || exit
2"` numa posição arbitrária (por exemplo, dentro de `"permissions"` ou num objeto `"metadata"` de
nível raiz).

**Gate que deveria pegar:** AC2 (teste de concordância falha se chamador novo passa arquivo inteiro)
— mas se o coletor modificado ainda varre todo o documento à procura da chave `"command"`, os 12
chamadores continuam sendo chamados com o arquivo inteiro e nenhum teste novo detecta que a
restrição não foi ao nível do objeto-pai.

**Direção:** falso negativo e falso positivo coexistem. O falso positivo (prosa com `"command":
"algum texto que contenha trackfw guard"`) persiste; o falso negativo (marcador soterrado num nível
correto) não é introduzido, mas o defeito original continua vivo para chaves diferentes de
`"command"` (como `"bash"` do Copilot global) e para qualquer campo que não seja `"command"` num
objeto-pai errado.

### 2.2 Ameaça B — Fix fecha falso positivo mas abre fail-open (prioritária)

O implementador percebe que `validateCredentialGuardPresenceRequired` conta prosa como fiação e
corrige isso. A função passa a navegar só pelo subárvore de hooks. Mas os chamadores que alimentam
`validateHookLineForm` (violation) continuam recebendo o arquivo inteiro, produzindo violations
falsas para `permissions.allow` e `autoMode.environment` — e o AC3 reprova.

**Cenário inverso (mais perigoso):** o implementador corrige os chamadores de forma
e deixa os chamadores de presença intactos, acreditando que "a presença já era restrita". O
fail-open (AC4) permanece sem teste, e o AC3 parece verde porque o caso de falso positivo testado
está no caminho de forma, não no de presença.

**Gate que deveria pegar:** teste de AC4 com fixture que coloca o marcador exato no campo errado
e verifica `rc == 1` (presença não encontrada). Sem esse teste, o auditor vê "AC3 verde" e aprova.

### 2.3 Ameaça C — Copilot global silenciado pelo fix (fail-open novo)

O implementador escreve um coletor que restringe a `"command"` por nome de chave. No Copilot
global (`~/.copilot/settings.json`), o hook usa `"bash"` em vez de `"command"`. O coletor novo
não encontra marcador nenhum e — dependendo da lógica de presença — ou passa sem aviso (fail-open
silencioso) ou gera uma falsa ausência (violation de presença onde o harness está correto).

**Gate que deveria pegar:** um teste de concordância que gere um settings.json de Copilot global
válido (com `"bash"`) e exija rc=0 sem violations. Esse teste reprova em `origin/main` porque o
campo `"bash"` não existe no branch corrigido se a restrição for apenas por chave `"command"`.

### 2.4 Ameaça D — Subtree restriction fecha #618 mas deixa AC4 aberto

O implementador lê "12 chamadores passam o arquivo inteiro" e corrige os 12 — trocando `parsed` por
`parsed["hooks"]`. O caso do autoMode.environment (#618) para de gerar aviso. O teste de AC3 passa.
O auditor marca o ML como concluído.

**O fail-open de AC4 permanece:** o fixture de Test A (marcador em `"note"` dentro do array
`hooks.pre_run_command`) continua produzindo `rc=0` porque `collectCommandsWithMarker` ainda percorre
todo objeto dentro do array, incluindo o campo `"note"`. A subtree restriction resolve um nível de
granularidade acima do defeito.

**Gate que deveria pegar:** o teste de AC4 (Test A / Windsurf) deve reprovar mesmo após a subtree
restriction ser aplicada. Se o teste de AC4 não for escrito, nenhum gate pega essa ameaça.

**Superfície de entrada:** qualquer hook entry com o marcador num campo auxiliar (e.g., `"name"`,
`"description"`, `"note"`, `"label"`) e o comando real em `"command"`. Todas as CLIs que suportam
campos auxiliares nomeados nos objetos de hook são vulneráveis.

### 2.5 Ameaça E — `anySubcmdFormFound` disparado por prosa

`validator_credential_guard.go:641`: quando `collectCommandsWithMarker` encontra um match não-exato
(a forma de subcomando, não a forma de shell script), define `anySubcmdFormFound = true`, que
dispara `guardBinaryProbeOnce`. Prosa que contenha a string `trackfw guard credential` (sem o
sufixo `|| exit 2`) num campo fora de hooks dispara a sonda do binário, adicionando latência e
possivelmente uma violation de sonda num ambiente onde o binário não está instalado no PATH esperado.

**Superfície:** qualquer `README.md` serializado num campo de string do settings, ou qualquer
texto de documentação inline que mencione o subcomando.

**Gate que deveria pegar:** o mesmo AC3, braço "fora de hooks não gera aviso". Mas um implementador
que corrija apenas os chamadores de presença e de forma pode deixar intacto o caminho que define
`anySubcmdFormFound`, e AC3 passa porque a violation de forma não é emitida — apenas a sonda é
disparada, que não produz erro visível na saída de validate.

---

## 3 — Alvos de falsificação

Para cada superfície: onde o sabotador entra, qual gate deveria pegar, em qual direção.

### 3.1 Windsurf — fail-open (AC4)

**Fixture medida (binário 11.0.1, tmp.DBWZGo20nC):**

```
.windsurf/hooks.json:
{
  "hooks": {
    "pre_run_command": [
      {
        "note": "trackfw guard credential || exit 2",
        "command": "echo x"
      }
    ],
    "pre_write_code": [
      {
        "note": "trackfw guard credential || exit 2",
        "command": "echo x"
      }
    ]
  }
}
```

Resultado: `rc=0`, 0 violations, 1 warning não-relacionado (`write_scope_not_wired`). O guard nunca
executa. `trackfw validate` passa completamente.

**Direção:** falso negativo — a regra de presença (`validateCredentialGuardPresenceRequired`)
encontra o marcador exato num campo `"note"` e conta como fiação existente. Gate atual: nenhum.
Gate que deve existir: teste AC4 que gera esta fixture e exige `rc != 0`.

**Falsificação em `origin/main`:** o AC4 deve ser verificado contra `origin/main` para confirmar que
o teste reprova antes da correção (braço "reprova em `origin/main`" exigido pelo AC3).

### 3.2 Amazon Q — fail-open (AC4)

**Fixture medida (mesmo tmp):**

```
.amazonq/cli-agents/q_cli_default.json:
{
  "hooks": {
    "execute_bash": {
      "matcher_1": {
        "hooks": [
          { "note": "trackfw guard credential || exit 2", "command": "echo x" }
        ]
      }
    },
    "fs_write": {
      "matcher_1": {
        "hooks": [
          { "note": "trackfw guard credential || exit 2", "command": "echo x" }
        ]
      }
    }
  }
}
```

Resultado: `rc=0`, 0 violations, 1 warning não-relacionado. Confirmação independente do mesmo
vetor.

### 3.3 Global `~/.claude/settings.json` — falso positivo em `autoMode.environment` (AC3, caso #618)

**Fixture medida:**

```json
{
  "autoMode": {
    "environment": [
      "trackfw-credential-guard.sh is referenced here for documentation"
    ]
  },
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          { "type": "command", "command": "trackfw guard credential || exit 2" }
        ]
      }
    ]
  }
}
```

Resultado: warning de forma legada (`references trackfw-credential-guard.sh`), `rc=0`. O aviso
instrui `trackfw update harness`, que não altera `autoMode.environment`. É o caso exato do issue
#618.

**Direção:** falso positivo — a string `trackfw-credential-guard.sh` numa lista de ambiente não é
um hook. Gate que deve existir: AC3 braço "fora de hooks não gera aviso", com esta fixture.

**Falsificação em `origin/main`:** o test AC3 (fora-de-hooks → sem aviso) deve reprovar em
`origin/main`.

### 3.4 Global `~/.claude/settings.json` — falso positivo em `autoMode.environment` (subcmd form)

**Fixture medida:**

```json
{
  "autoMode": {
    "environment": ["trackfw guard credential is used here"]
  },
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          { "type": "command", "command": "trackfw guard credential || exit 2" }
        ]
      }
    ]
  }
}
```

Resultado: violation (`missing type:command` para a entrada em `autoMode.environment`), `rc=1`. Mais
grave que o caso .sh: bloqueia validate mesmo com o hook correto instalado.

### 3.5 Projeto `.claude/settings.json` — falso positivo em `permissions.allow`

**Fixture medida:**

```json
{
  "permissions": {
    "allow": ["Bash(trackfw guard credential --check)"]
  },
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          { "type": "command", "command": "trackfw guard credential || exit 2" }
        ]
      }
    ]
  }
}
```

Resultado: violation (`missing type:command`), `rc=1`. A string em `permissions.allow` não é um
hook; o validador trata como comando de hook malformado.

### 3.6 Amazon Q — estrutura real vs fixture de Test B

O Test B (confirmação do fail-open) usou uma fixture com `"execute_bash"` e `"fs_write"` como
chaves diretas sob `"hooks"`. A estrutura real do Amazon Q é `hooks.preToolUse[{matcher:"execute_bash",hooks:[…]}]`.
O Test B ainda produziu `rc=0` porque os callers de arquivo-inteiro (L512, L583 etc.) encontraram
o marcador em `"note"` independentemente da estrutura. A confirmação de fail-open é válida, mas
o mecanismo específico medido foi via callers de arquivo-inteiro, não via L773.

**Alvo de falsificação pós-fix:**
- **Falso negativo (over-restrictive fix):** após a correção, um coletor que restrinja corretamente
  ao `obj["hooks"][].command` dentro do matcher Amazon Q não deve perder o marcador legítimo quando
  ele está no campo `"command"` na posição correta. Fixture de concordância deve verificar isso.
- **Falso negativo (Ameaça D):** marcador em `"note"` dentro do `obj["hooks"]` array (estrutura
  correta) com `"command": "echo x"` deve produzir violation de presença após o fix.

**Medição pendente:** medir o fail-open via L773 com fixture de estrutura correta
(`hooks.preToolUse[{matcher:"execute_bash",hooks:[{note:"…",command:"echo x"}]}]`). Pode ser
feito no ML-1A como parte da fixture de AC4.

### 3.7 Kiro — in-entry fail-open (Ameaça D, nested schema)

Kiro usa `hooks[].action.command` com `action.type:"command"`. Um campo `action.name` ou
`action.description` dentro do mesmo objeto poderia conter o marcador.

**Alvo de falsificação pós-fix:**
- **Falso negativo:** coletor over-restrictive que restrinja a `action.command` mas não encontre o
  marcador quando ele está no campo `action.command` legítimo.
- **Fail-open:** marcador em `action.name` com `action.command:"echo x"` deve produzir violation.

**Medição pendente:** incluir no conjunto de fixtures de AC4 no ML-1A.

### 3.8 Copilot global — assimetria `bash` vs `command` (Ameaça C)

A assimetria é documentada pela leitura de `globalGuardConfigFiles` (L91) e do gerador
correspondente. A medição do fail-open e do comportamento pós-fix para este caminho requer uma
fixture em `t.TempDir()` com HOME sintético — pode ser feita no ML-1A como parte do conjunto
de AC4 (fixture com `"bash"` no campo de comando e `"note"` com o marcador).

**Alvos de falsificação:**
- **Falso negativo pós-fix:** após a correção, se o coletor novo restringe a `"command"` e o
  arquivo Copilot global usa `"bash"`, a regra de presença não encontra o marcador → violation de
  presença onde o harness está correto.
- **Gate:** teste de concordância que gere fixture Copilot global válida (com `"bash"`) e exija
  `rc=0`.

### 3.9 `anySubcmdFormFound` disparado por prosa (Ameaça E)

Qualquer campo de string no arquivo que contenha a substring `trackfw guard credential` (sem
`|| exit 2`) define `anySubcmdFormFound = true` no chamador de `validateCredentialGuardHookForm`,
disparando `guardBinaryProbeOnce`. Não gera violation imediata, mas adiciona latência e pode
produzir violation de binário.

**Alvo de falsificação:**
- **Falso negativo:** o implementador corrija presença e forma mas deixe o path `anySubcmdFormFound`
  lendo do arquivo inteiro. O AC3 passa (sem violation de forma para prosa), mas prosa ainda dispara
  a sonda.
- **Gate:** não há teste atual para esta condição. Deve ser adicionado em AC3: "fora de hooks não
  dispara binary probe".

---

## 4 — Residual declarado e recomendações para Wave 1

Este desenho aceita não cobrir os seguintes pontos:

**R1 — Windsurf global (`~/.codeium/windsurf/hooks.json`) não validado.**
O arquivo não está em `globalGuardConfigFiles`. O guard global Windsurf não é verificado pelo
`trackfw validate`. Esta lacuna é pré-existente a este defeito. A correção deste roadmap não a
fecha. Requer REQ própria se for priorizada.

**R2 — Copilot global: medição do in-entry fail-open delegada ao ML-1A.**
A assimetria `bash` vs `command` foi documentada por leitura de código. A medição do fail-open
específico (marcador em `"note"` dentro de hook entry Copilot global com `"bash"` como chave de
comando) requer HOME sintético — não feita nesta Wave 0 por ser tecnicamente idêntica ao Test A
(mesmo mecanismo, campo `"note"`, coletor recursivo). O ML-1A deve incluir esta fixture em
`t.TempDir()` como parte dos testes de AC4. Não é residual de design — é medição pendente.

**R3 — `anySubcmdFormFound` side effect: sem teste de sonda.**
O path que dispara `guardBinaryProbeOnce` por prosa não tem cobertura de teste. A correção de AC3
elimina o falso positivo de forma e de presença; o teste de probe pode ser adicionado como best
effort no mesmo ML, mas não é bloqueante para AC3 e AC4.

**R4 — `scripts/check-gates-falsify.sh` não precisa mudar.**
Os `grep -qF` nos scenarios 5566/5577/5606/5677/5835–5888 verificam o output do gerador, que sempre
escreve no campo de comando correto. Após a correção, esses greps continuam passando. Nenhuma
alteração necessária.

**R5 — `scripts/windows-repro/guard-global-home.ps1`.**
Script de reprodução, não de validação. Não faz parte do CI. Fora de escopo.

### 4.6 Recomendação de `Files affected` para a Wave 1

Mínimo necessário para AC1–AC5:

| Arquivo | Ação |
|---|---|
| `internal/validator/validator_credential_guard.go` | Novo coletor schema-aware; corrigir todos os 8 chamadores (incluindo L758 e L773 via `extractCredentialGuardCommandsFromEvent`) |
| `internal/validator/validator_git_branch_guard.go` | Corrigir todos os 6 chamadores |
| `internal/validator/validator_credential_guard_test.go` | Testes de AC3 (falso positivo em `autoMode`, `permissions.allow`) e AC4 (fail-open Windsurf e Amazon Q) |
| `internal/validator/validator_guard_hook_concordance_external_test.go` | Teste de concordância Copilot global com `"bash"` (contra Ameaça C) |
| `docs/cli-parity.md` | Adicionar mapa CLI → campo de comando (tabela da seção 1.2 deste documento) |

O novo coletor deve ser parametrizado por esquema do CLI (`commandKey` + `requiredDepth` em vez de
busca recursiva por substring em toda a árvore). Para cada arquivo de hook, a função recebe o
array exato do evento (como L758/L773 já fazem) E só aceita o campo de chave correto dentro de
cada objeto-entry. Isso fecha AC1 e AC2 simultaneamente:
- AC2 pode ser verificado por um teste que chama o coletor com uma árvore inteira e confirma que
  ele rejeita, ou por um linter que proíbe `parsed` diretamente nos chamadores de presença/forma.
- Para Copilot global (`"bash"` em vez de `"command"`): a instância do coletor para
  `~/.copilot/settings.json` deve usar `commandKey = "bash"`.

**Critério de AC3 (braço que reprova em `origin/main`):** tanto o teste de "fora de hooks não gera
aviso" quanto o teste de "campo não-command dentro de hook não conta como fiação" devem reprovar em
`origin/main`. Verificar os dois braços como parte da aceitação do ML-1A.

### 5.1 Testes que afirmam o comportamento atual e precisam mudar

Os seguintes testes passam hoje pelo comportamento incorreto e devem ser revisados no ML-1A:

- Concordância em `validator_guard_hook_concordance_external_test.go`: testes que passam output do
  gerador com fixtures de campo de comando correto — continuarão passando após o fix (o gerador
  escreve no campo certo). Verificar que nenhum usa fixture com marcador em campo auxiliar.
- `TestCredentialGuardHookResolvable_KiroNaoGeraFalsoPositivoComCamposVizinhos` em
  `validator_credential_guard_test.go`: passes hoje porque os campos `"name"/"description"` do
  fixture Kiro não contêm a string `.sh`. Continuará passando. Nenhuma mudança necessária.
- Testes de presença em `validator_credential_guard_test.go:1146–1278` (`validateCredentialGuardPresenceRequired`)
  que usam `windsurfHooksWithSubcmd()`: o fixture escreve o comando no caminho correto de `"command"`.
  Continuarão passando. Verificar que nenhuma fixture coloca o marcador em campo auxiliar.
- **Novos testes obrigatórios (AC3 e AC4):** pelo menos um teste com marcador em campo auxiliar fora de
  hooks (para AC3/FP) e um com marcador em campo auxiliar dentro de hook (para AC4/fail-open), um
  por família de CLI (Windsurf plano, Kiro aninhado, Amazon Q com matcher).
