---
date: 2026-10-25
roadmap: ROADMAP-2026-10-25-validate-casa-o-marcador-do-guard-em-qualquer-string-do-settings-e-nao-so-em-comando-de-hook.md
ml: ML-2A
agente: hefesto-tf
req: docs/req/REQ-2026-10-25-validate-casa-o-marcador-do-guard-em-qualquer-string-do-settings-e-nao-so-em-comando-de-hook.md
---

# Parecer de Qualidade — ML-2A (D16, REQ-2026-10-25)

**Veredito: REPROVADO**

Três achados bloqueantes: (B1) violação da Regra Dura de Reconciliação — comentário contraditório no
teste CopilotGlobal; (B2) linha Windsurf ausente da tabela global em `docs/cli-parity.md`; (B3) três
testes novos ausentes do contra-braço da auditoria do ML-1A. Três recomendações.

---

## Escopo inspecionado

`git diff origin/main..HEAD -- internal/ docs/cli-parity.md`

Arquivos alterados: `validator_credential_guard.go`, `validator_git_branch_guard.go`,
`validator_guard_wiring.go`, `validator_credential_guard_test.go`,
`validator_git_branch_guard_test.go`, `validator_guard_command_schema_test.go` (novo),
`validator_guard_d16_wiring_external_test.go` (novo),
`validator_guard_hook_concordance_external_test.go`, `export_test.go`, `docs/cli-parity.md`.

---

## Item 1 — Fonte única do esquema por CLI?

Há dois mapas, mas cobrem populações distintas:

- `guardWiringHookLayout` (`validator_guard_wiring.go:90`) — arquivos de projeto
  (`credentialGuardHookFiles`).
- `guardGlobalHookLayout` (`validator_git_branch_guard.go:126`) — arquivos globais
  (`globalGuardConfigFiles`).

Cada mapa tem um teste de cobertura próprio:

```
TestGuardWiringHookLayout_CobreTodasAsEntradas   → validator_guard_wiring_test.go:1872
TestGuardGlobalHookLayout_CobreTodasAsEntradas   → validator_guard_command_schema_test.go:34
```

Ambos passam na branch. Um novo CLI adicionado a `credentialGuardHookFiles` sem entrada em
`guardWiringHookLayout` é capturado por `TestGuardWiringHookLayout_CobreTodasAsEntradas`. Idem
para o lado global.

**Nota sobre Amazon Q:** o campo `requiresCommandType=false` em `credentialGuardHookFiles` reflete
que o schema Amazon Q não tem discriminante `type` nos objetos de hook internos. Evidência em
`validator_credential_guard.go:106`:

```
requiresCommandType=false (schema has no "type" discriminant in inner command objects)
```

Portanto a assimetria entre `credential_guard_hook_resolvable` (`requiresCommandType=false`) e
`guardWiringExtractCommandsFromEntry` não constitui fail-open do mesmo mecanismo que #618: não existe
campo `type` no schema Amazon Q a ser interpretado incorretamente. A divergência entre os dois
extratores é correta por construção para este CLI.

---

## Item 2 — Código morto e duplicado

`collectCommandsWithMarker` (o coletor recursivo removido pela D16) não existe mais no pacote.
Evidência — o teste `TestGuardRecursivoAusente_CodigoFonte` usa `forbidden = "func
collectCommandsWithMarker("` e passa (ver Item 3a). Busca com a mesma string literal confirma que
a única ocorrência é essa constante:

```
$ grep -rnF 'func collectCommandsWithMarker(' internal/
internal/validator/validator_guard_command_schema_test.go:74:	const forbidden = "func collectCommandsWithMarker("
```

`collectCommandsFromCommandKeyArray` permanece e está em uso em `validator_credential_guard.go`
(linhas 838 e 855 via `extractCredentialGuardCommandsFromEvent`). Não é código morto.

Nenhum outro símbolo órfão encontrado.

---

## Item 3 — Testes

### 3a — `TestGuardRecursivoAusente_CodigoFonte` (critério de estabilidade)

O teste procura a string literal `func collectCommandsWithMarker(`. Uma reintrodução com nome
diferente não seria detectada. Os testes AC3/AC4 são a barreira comportamental real (ver R1).

Todos os testes AC3/AC4 passam:

```
TestCredentialGuardHookResolvable_PermissionsAllowNaoGeraViolation   PASS
TestCredentialGuardHookResolvable_EnvMarkerNaoGeraViolation          PASS
TestCredentialGuardPresenceRequired_Windsurf_MarkerSoEmNote_Violation PASS
TestCredentialGuardPresenceRequired_AmazonQ_MarkerSoEmNote_Violation  PASS
```

### 3b — `TestGuardWiringRemoved_MutacaoDoComandoParaEchoX` (pin declarado)

O comentário de Reconciliação (`validator_guard_d16_wiring_external_test.go:169`) declara: "este arm
também passa em origin/main (pin de regressão)." Passa nos 8 CLIs. Declaração correta.

### 3c — `TestGuardGlobalHookResolvable_CopilotGlobal_MarkerEmNoteNaoContaComoFiacao` — ver B1.

### 3d — Varredura de testes novos — ver B3.

---

## Item 4 — `docs/cli-parity.md`

A tabela adicionada para o escopo global (`guardGlobalHookLayout`) omite Windsurf. Ver achado B2.

O `gate=` da anotação `trackfw-contract` aponta para
`internal/validator/validator_guard_command_schema_test.go`, que existe em disco. O script
`scripts/check-parity-contract-coverage.sh` valida: existência do arquivo nomeado no `gate=`,
formato da anotação e ausência de valores vazios em chaves declaradas. Não verifica nomes de testes
no campo `partial=` — apenas que o valor não é vazio (confirmado lendo o script). A anotação é
sintaticamente válida; o `make quality` não falha por causa dela.

---

## Item 5 — Windsurf global em HOME isolado

Binário compilado da branch, testado em `mktemp -d` com `HOME` e `USERPROFILE` isolados,
`CLAUDE_CONFIG_DIR=""`.

**Forma atual (CredentialGlobalCurrentPSPOSIX):** rc=0, zero violations. Confirmado.

**Cenário #618 (autoMode.environment com marcador, hook correto presente):** rc=0, sem menção a
autoMode.environment. O falso positivo original está corrigido.

**Formas legadas:** rc=0 com warnings de migração esperados em todas as variantes.

---

## git status (medido após escrita deste arquivo)

```
?? docs/qualidade/2026-10-25-revisao-marcador-fora-de-hook.md
?? docs/seguranca/2026-10-25-redteam-marcador-fora-de-hook.md
```

---

## Achados

### B1 — Reconciliação contraditória em `TestGuardGlobalHookResolvable_CopilotGlobal_MarkerEmNoteNaoContaComoFiacao`

**Arquivo:linha:** `internal/validator/validator_guard_hook_concordance_external_test.go:190`

**Problema:** três inconsistências no mesmo teste, que juntas constituem violação da Regra Dura de
Reconciliação:

1. **Nome vs. fixture:** o nome diz `MarkerEmNote` mas a fixture usa a chave `"cwd"` (linha 224:
   `"cwd": "trackfw-credential-guard.sh"`), não `"note"`.

2. **Reconciliação contradiz o roadmap:** o comentário afirma "Old code (recursive search) would
   find the marker in 'cwd', try to check if a script with that path exists, and emit a violation."
   O roadmap (seção de auditoria do ML-1A, linha 84) declara que `CopilotGlobal_MarkerEmNote` é uma
   "fixação de regressão declarada" que passa em `origin/main`. As duas afirmações são incompatíveis:
   se o código antigo emitiria uma violation, o teste teria de reprovar em `origin/main`, não passar.

3. **Ausência de declaração de pin:** diferentemente de `TestGuardWiringRemoved_MutacaoDoComandoParaEchoX`
   (que diz explicitamente "este arm também passa em origin/main (pin de regressão)"), este teste não
   declara que é um pin. Um leitor futuro lê o comentário e infere que o teste prova D16.

**Qual lado está errado não foi medido nesta revisão.** Dois caminhos:

- Verificar o comportamento do código antigo com a chave `"cwd"` via
  `git show origin/main:internal/validator/validator_git_branch_guard.go` e confirmar se a violation
  era de fato emitida em `origin/main`.
- Se o roadmap está certo (o teste é um pin): corrigir o comentário de Reconciliação para declarar
  explicitamente "este teste é um pin de regressão — passa em origin/main" e adicionar à lista
  "Passam lá" do roadmap.
- Se o comentário está certo (falha em origin/main): remover da lista "Passam lá" do roadmap e
  corrigir o nome para `MarkerEmCwd`.

Somente um dos dois pode estar certo. A contradição bloqueia.

---

### B2 — Linha Windsurf ausente da tabela global em `docs/cli-parity.md`

**Arquivo:** `docs/cli-parity.md`, seção `Escopo global — guardGlobalHookLayout` adicionada neste ML.

**Problema:** a tabela lista os CLIs com entrada em `guardGlobalHookLayout` mas omite Windsurf
(`.codeium/windsurf/hooks.json` → `hookCmdSchemaFlat`), adicionado a `guardGlobalHookLayout` e
`globalGuardConfigFiles` neste mesmo ML. Evidência — `validator_git_branch_guard.go:135`:

```go
".codeium/windsurf/hooks.json": hookCmdSchemaFlat,
```

A anotação `trackfw-contract` do bloco (`partial=`) também não menciona os testes Windsurf
(`TestGuardGlobalHookResolvable_Windsurf_CurrentForm_ZeroViolation`,
`TestGuardGlobalHookResolvable_Windsurf_MissingScript_EmitsViolation`).

**Correção proposta:** acrescentar linha na tabela para Windsurf com path
`~/.codeium/windsurf/hooks.json`, schema `hookCmdSchemaFlat` e campo `entry["command"]`. Atualizar
o campo `partial=` da anotação para incluir os dois testes de Windsurf global.

---

### B3 — Três testes novos ausentes do contra-braço da auditoria do ML-1A

**Arquivo:** roadmap, seção de auditoria do ML-1A (listas "Passam lá" e "Reprovam lá").

**Varredura:** `git diff origin/main..HEAD -- internal/ | grep '^+func Test'` lista 16 novos
testes. Cruzados contra as listas do roadmap (mais a nota "sem o teste de layout, que usa símbolo
novo"), três ficam sem classificação:

| Teste | Reconciliação própria no teste | Ausente de |
|---|---|---|
| `TestGuardWiringRemoved_ControlArm_OrigemIntacta` | "pin de regressão — arm-de-controle=0 vale também em origin/main" (`validator_guard_d16_wiring_external_test.go:112`) | "Passam lá" |
| `TestGuardGlobalHookResolvable_Windsurf_CurrentForm_ZeroViolation` | "was absent from globalGuardConfigFiles before ML-1A2" — nova funcionalidade, falha em origin/main (`validator_guard_d16_wiring_external_test.go:483`) | "Reprovam lá" |
| `TestGuardGlobalHookShLegacyWarnings_MarkerNaHook_EmiteAviso` | "a função detecta a forma D2 em escopo global — a lacuna que o ML-6B deixou em aberto" (`validator_git_branch_guard_test.go:1392`) | nenhuma das duas listas |

Cada teste tem Reconciliação própria. O problema é que a auditoria do ML-1A não os classifica,
tornando impossível ao arquiteto verificar o cruzamento sem os candidatos completos (Regra Dura de
Reconciliação: "a auditoria do arquiteto verifica esse cruzamento, não só se o teste passa").

**Nota sobre `TestGuardGlobalHookShLegacyWarnings_MarkerNaHook_EmiteAviso`:** o comentário refere
"ML-6B" (REQ anterior). Não foi medido se o teste passa em `origin/main`. A classificação correta
é responsabilidade do ML-1A.

**Correção proposta:** completar a auditoria do roadmap adicionando os três testes às listas
"Passam lá" ou "Reprovam lá" com a evidência correspondente.

---

### R1 — Critério de `TestGuardRecursivoAusente_CodigoFonte` é frágil a renomeação

**Arquivo:linha:** `internal/validator/validator_guard_command_schema_test.go:74`

**Problema:** o critério `const forbidden = "func collectCommandsWithMarker("` captura apenas aquele
nome exato. Uma reintrodução com nome diferente não é detectada.

**Correção proposta:** acrescentar no comentário: "Nota: este teste é um pin de nome, não um gate
comportamental. Os testes AC3 e AC4 (`Windsurf_MarkerSoEmNote_Violation`,
`AmazonQ_MarkerSoEmNote_Violation`, `PermissionsAllowNaoGeraViolation`) são a barreira real — um
coletor recursivo com outro nome quebraria esses testes antes de chegar aqui."

---

### R2 — `guardWiringExtractCommandsFromEntry` não tem caso para `hookCmdSchemaFlatBash`

**Arquivo:linha:** `internal/validator/validator_guard_wiring.go:387`

**Problema:** o `switch` trata `hookCmdSchemaNested`, `hookCmdSchemaFlat` e `hookCmdSchemaKiro`,
mas não `hookCmdSchemaFlatBash`. O `return nil` final é silencioso. Se uma entrada
`hookCmdSchemaFlatBash` for adicionada a `guardWiringHookLayout`, o extrator silenciosamente não
encontrará nenhum comando (falso negativo em `guard_wiring_removed`). Atualmente seguro porque
`guardWiringHookLayout` não tem entradas `hookCmdSchemaFlatBash`, mas a constante está definida no
mesmo arquivo.

**Correção proposta:** adicionar caso explícito com comentário explicando que `hookCmdSchemaFlatBash`
é escopo global apenas; para fail-closed, usar panic se o caso for atingido por
`guardWiringHookLayout`.

---

### R3 — Comentários com contagem "6" obsoletos em `validator_git_branch_guard.go`

**Arquivo:linhas:** `internal/validator/validator_git_branch_guard.go:167, 272`

**Problema:** após adicionar Windsurf, `globalGuardConfigFiles` tem 7 entradas. Dois comentários
ainda dizem "6":

- Linha 167: `"for each of the 6 globalGuardConfigFiles that exists AND"`
- Linha 272: `"kind of closed, enumerable list (6 entries)"`

(Linha 698 descreve o estado anterior ao ML — "6" está correto lá.)

**Correção proposta:** atualizar linhas 167 e 272 para "7".

---

## Verificações de saída

```
go test -run 'TestGuardWiringHookLayout_CobreTodasAsEntradas|TestGuardGlobalHookLayout_CobreTodasAsEntradas|TestGuardRecursivoAusente_CodigoFonte' ./internal/validator/
PASS (0.26s)

go test -run 'TestGuardWiringRemoved_MutacaoDoComandoParaEchoX' ./internal/validator/
PASS (1.91s) — 8 CLIs

go test -run 'TestGuardGlobalHookResolvable_CopilotGlobal_MarkerEmNoteNaoContaComoFiacao' ./internal/validator/
PASS (0.09s) — passa; comentário de Reconciliação contradiz o roadmap (B1)

go test -run 'TestGuardGlobalHookResolvable_Windsurf' ./internal/validator/
PASS (0.09s) — 2 casos

go test -run 'TestCredentialGuardPresenceRequired_Windsurf_MarkerSoEmNote|TestCredentialGuardPresenceRequired_AmazonQ_MarkerSoEmNote' ./internal/validator/
PASS (0.10s)

go test -run 'TestGuardGlobalHookShLegacyWarnings_MarkerEmAutoModeEnvironment|TestGuardGlobalHookShLegacyWarnings_AutoModeComHookCorreto' ./internal/validator/
PASS (0.12s)

go test -run 'TestCredentialGuardHookResolvable_PermissionsAllowNaoGeraViolation|TestCredentialGuardHookResolvable_EnvMarkerNaoGeraViolation|TestCredentialGuardHookResolvable_PermissionsAllowDecoyRealHookAindaDetectado' ./internal/validator/
PASS (0.12s)

go build ./... — OK
```
