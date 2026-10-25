---
date: 2026-10-25
req: "docs/req/REQ-2026-10-25-validate-avisa-quando-o-arquivo-de-hook-de-projeto-nao-tem-o-credential-guard-nos-clis-sem-regra-de-presenca.md"
roadmap: "ROADMAP-2026-10-25-validate-avisa-quando-o-arquivo-de-hook-de-projeto-nao-tem-o-credential-guard-nos-clis-sem-regra-de-presenca.md"
ml: "ML-2A"
agente: hefesto-tf
---

# Parecer de qualidade — aviso de presença do credential guard (D17, #619)

> Veredito: **APROVADO** — 0 achados bloqueantes (B), 3 recomendações (R)

Diff examinado: `git diff origin/main..HEAD -- internal/ scripts/ docs/cli-parity.md`
Arquivos lidos: roadmap (ML-0A + ML-1A auditorias), ADR D17, `validator_credential_guard.go`,
`validator_credential_guard_presence_warning_test.go`, `validator_guard_d16_wiring_external_test.go`,
`validator_guard_hook_concordance_external_test.go`, `export_test.go`, `validator_guard_wiring.go`,
`cli-parity.md`, `scripts/check-gates-falsify.sh`.

---

## Verificações do escopo

### 1. Fonte única — `credentialGuardRequiredEntries` duplica algum fato já em `guardWiringHookLayout`?

`credentialGuardRequiredEntries` armazena um campo `schema hookCmdSchemaKind` por entrada. O
`guardWiringHookLayout` (em `validator_guard_wiring.go:90`) já mapeia `path → schema` para os mesmos
arquivos. Os valores são atualmente idênticos para as 6 entradas `warnOnly=true`:

| path | `guardWiringHookLayout` | `credentialGuardRequiredEntries.schema` |
|---|---|---|
| `.claude/settings.json` | `hookCmdSchemaNested` | `hookCmdSchemaNested` |
| `.codex/hooks.json` | `hookCmdSchemaNested` | `hookCmdSchemaNested` |
| `.gemini/settings.json` | `hookCmdSchemaNested` | `hookCmdSchemaNested` |
| `.github/hooks/trackfw-attention.json` | `hookCmdSchemaFlat` | `hookCmdSchemaFlat` |
| `.cursor/hooks.json` | `hookCmdSchemaFlat` | `hookCmdSchemaFlat` |
| `.kiro/hooks/trackfw-attention.json` | `hookCmdSchemaKiro` | `hookCmdSchemaKiro` |

Não há teste que pina esta consistência. Se `guardWiringHookLayout` for atualizado (ex.: formato de
hook de um CLI muda) sem atualizar `credentialGuardRequiredEntries`, `extractPresenceCommandsD17`
usaria o schema errado silenciosamente. → **R1**

### 2. `extractPresenceCommandsD17` repete a lógica de `collectCommandsWithMarkerBySchema`?

`collectCommandsWithMarkerBySchema` (`validator_credential_guard.go:351`) itera **todos** os eventos
do documento. `extractPresenceCommandsD17` (`validator_credential_guard.go:1014`) itera um único
slot `(event, matcher)`. A diferença semântica é intencional: D17 precisa de granularidade por par,
não por documento.

No entanto, o código de travessia JSON para os 3 schemas (Nested, Flat, Kiro) é **estruturalmente
duplicado** nas duas funções. Se o formato de um schema mudar em `collectCommandsWithMarkerBySchema`
(ex.: D16 adicionar novo campo), a mesma mudança precisa ser feita em `extractPresenceCommandsD17` —
e não há teste que detecte a divergência. → **R2**

### 3. Testes

**Nomes e reconciliações:**
Todos os testes em `validator_credential_guard_presence_warning_test.go` têm reconciliação na
docstring e o vínculo ao par (hookEvent, matcher) que afirmam. Verificados contra o corpo:
consistentes.

**Erro de grafia — bloqueante cosmético:**
`internal/validator/validator_guard_d16_wiring_external_test.go:786`:
```
func TestCredentialGuardPresenceRequired_Kiro_NaoTemViolacaoDePrescaDisco(t *testing.T) {
```
"PrescaDisco" é grafado como `Pre-s-c-a-Disco` (faltam os dois caracteres "en"); o correto é
`PresencaDisco`. O comentário imediatamente acima (linha 777) cita `NaoTemRegraDePresencaDisco` —
nome diferente do da função, e também semanticamente desatualizado após D17 (Kiro agora tem regra de
presença `warnOnly=true`). O teste roda e passa (verificado com `go test -run`); o erro é somente de
nomenclatura. → **R3**

**Teste vacuoso — nenhum:**
Os 17 testes de `validator_credential_guard_presence_warning_test.go` foram avaliados. Cada um
injeta ou omite pares específicos e afirma contagens não-triviais. Nenhum é trivialmente verdadeiro.
O teste `TestCredentialGuardPresenceWarning_ArquivoAusente` (fixture com diretório vazio) é o caso
de silêncio esperado pela D17; não é vacuoso — garante que ausência de arquivo não gera aviso.

**`export_test.go` — 7 novos exports:**
Todos os 7 exports adicionados são usados por testes:
- `ValidateCredentialGuardPresenceWarningsForTest` — usado em
  `validator_guard_hook_concordance_external_test.go:499` e
  `validator_guard_d16_wiring_external_test.go:815`.
- `CredentialGuardPresenceWarnOnlyCountForTest` — usado em `:592`.
- `CredentialGuardPresenceViolationCountForTest` — usado em `:596`.
- `PresenceWarnOnlyEventForTest`, `PresenceWarnOnlyEntryForTest`,
  `CredentialGuardPresenceWarnOnlyEntriesForTest` — usados pelo teste AC5 bidirectional em `:790`.
- `ValidateForTest` — usado em `validator_guard_hook_concordance_external_test.go:956`.

Nenhum export é desnecessário.

**`go test -run`** executados sem falhas:
```
TestCredentialGuardPresenceWarning_* (17 casos)   → PASS
TestCredentialGuardPresenceRequired_Kiro_NaoTemViolacaoDePrescaDisco → PASS
TestCredentialGuardPresence_AC5_EntryTableCount → PASS
TestCredentialGuardPresence_AC5_GeneratorTableBidirectional_GeneratedSubsetOfTable (6 subtestes) → PASS
```

### 4. `docs/cli-parity.md`

A seção `## Regra credential_guard_presence_warning` foi adicionada ao final do arquivo. A tabela
de pares (hookEvent + matcher) foi conferida contra `credentialGuardRequiredEntries` para todos os
6 CLIs: **consistente**. A anotação `trackfw-contract` está presente e cita corretamente os arquivos
de gate. O comportamento de silêncio e o "ponto único" estão documentados.

### 5. `scripts/check-gates-falsify.sh` — cenário 47

A função `s47_write_claude_guard_hook` foi atualizada de um único par `{matcher:"Bash"}` para os 3
pares que o D17 exige (`Bash|PowerShell`, `Read`, `Write|Edit`). O cenário 47 mede a regra
`credential_guard_hook_resolvable` (script ausente vs. presente), não a nova regra de presença.

A atualização é correta: com o fixture antigo (só `Bash`), o braço baseline (T47_OK) passaria com
`rc=0` porque a regra resolvable não muda — mas a nova regra D17 emitiria avisos de presença sobre
os 2 pares ausentes. O fixture atualizado silencia também os avisos de presença, alinhando a
semântica de "árvore íntegra" com o estado que um `trackfw update` real geraria.

O `$S47_MSG_MISSING` (`'but the script does not exist'`) é distinto da mensagem D17
(`'has no credential guard entry in event'`); as duas regras são independentes. O cenário
permanece medindo o que anuncia.

---

## Achados

### R1 — Sem teste de consistência entre `guardWiringHookLayout` e `credentialGuardRequiredEntries.schema`

**Arquivo:linha:** `internal/validator/validator_guard_wiring.go:90` e
`internal/validator/validator_credential_guard.go:822`

**Comportamento observado:** o campo `schema` das entradas `warnOnly=true` é declarado
independentemente do `guardWiringHookLayout`. A tabela `TestGuardWiringHookLayout_CobreTodasAsEntradas`
(linha 1872 em `validator_guard_wiring_test.go`) garante que todo path em `credentialGuardHookFiles`
tem entrada em `guardWiringHookLayout`, mas não garante que o schema da entry em
`credentialGuardRequiredEntries` coincida com o do layout.

**Risco:** mudança em `guardWiringHookLayout` não propagada para `credentialGuardRequiredEntries`
causa travessia silenciosamente errada em `extractPresenceCommandsD17`, produzindo falsos negativos
na regra de presença.

**Correção proposta:** adicionar um teste (ex.:
`TestPresenceWarnOnlyEntries_SchemaMatchesWiringLayout`) em
`internal/validator/validator_guard_wiring_test.go` que itera as entradas `warnOnly=true` de
`credentialGuardRequiredEntries` e afirma `entry.schema == guardWiringHookLayout[entry.hookFile.path]`
para cada uma.

---

### R2 — Travessia JSON de 3 schemas duplicada entre `extractPresenceCommandsD17` e `collectCommandsWithMarkerBySchema`

**Arquivo:linha:** `internal/validator/validator_credential_guard.go:1014` vs. `:351`

**Comportamento observado:** ambas as funções contêm cases `hookCmdSchemaNested`, `hookCmdSchemaFlat`
e `hookCmdSchemaKiro` com lógica estruturalmente idêntica de navegação JSON. Diferem apenas em que
`extractPresenceCommandsD17` filtra por `(event, matcher)` antes de entrar no loop interno.

**Risco:** uma correção de parser em `collectCommandsWithMarkerBySchema` (ex.: D16 passa a exigir
novo campo) precisaria ser replicada manualmente em `extractPresenceCommandsD17`; sem teste de
paridade, a divergência pode passar despercebida.

**Correção proposta (para refactor posterior):** extrair os loops internos de cada schema como
helpers privados recebendo filtros opcionais de event/matcher; ambas as funções chamam os helpers.
O custo é uma refatoração que toca múltiplos callers de `collectCommandsWithMarkerBySchema`; não é
urgente enquanto os schemas estiverem estáveis.

---

### R3 — Typo e mismatch comentário/função no teste Kiro de violation

**Arquivo:linha:** `internal/validator/validator_guard_d16_wiring_external_test.go:786`

**Comportamento observado:**
```go
// linha 777: TestCredentialGuardPresenceRequired_Kiro_NaoTemRegraDePresencaDisco afirma: ...
func TestCredentialGuardPresenceRequired_Kiro_NaoTemViolacaoDePrescaDisco(t *testing.T) {
```
Dois problemas independentes:
1. **Typo na função:** `NaoTemViolacaoDePrescaDisco` — "Presca" está truncado (falta "en"); deveria
   ser `NaoTemViolacaoDePresencaDisco`.
2. **Mismatch comentário ↔ função:** o comentário cita `NaoTemRegraDePresencaDisco` enquanto a
   função diz `NaoTemViolacao...`. Após D17, o nome do comentário também está desatualizado: Kiro
   agora tem regra de presença (`warnOnly=true`); o que a função afirma corretamente é que não há
   *violation* para Kiro.

**Correção proposta:** renomear a função para
`TestCredentialGuardPresenceRequired_Kiro_NaoTemViolacaoDePresencaDisco` e atualizar o comentário
(linha 777) para `NaoTemViolacaoDePresencaDisco`.

---

## Observação sobre cobertura do AC3 nos 4 CLIs restantes

A auditoria do ML-1A registra que o AC3 (guard global não silencia o aviso) é coberto por testes
apenas para Claude e Kiro; nos outros 4, a cobertura é pela medição da Wave 0 (contagem idêntica
com e sem guard global). A ausência de testes automatizados para Codex, Gemini, Cursor e Copilot
no AC3 não é um bloqueio desta ML (a medição da Wave 0 é evidência válida), mas é gap de cobertura
a considerar numa iteração futura.
