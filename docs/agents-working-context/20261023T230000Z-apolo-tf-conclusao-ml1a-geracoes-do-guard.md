# Contexto — apolo-tf — conclusão do ML-1A (gerações do guard, round 2)

**Data:** 2026-10-23T23:00:00Z
**REQ:** REQ-2026-10-23-update-nao-remove-geracoes-antigas-do-credential-guard...
**Roadmap:** ROADMAP-2026-10-23-update-nao-remove-geracoes-antigas-do-credential-guard (ML-1A pendente de auditoria)
**Branch:** fix/update-nao-remove-geracoes-antigas-do-credential-guard

## Correções aplicadas neste ciclo (após revisão do advisor)

### AC5 — userCmd agora contém "trackfw guard credential" como substring
- Antes: `"my-custom-tool --some-flag"` (não colide com Contains — não provava exatidão)
- Depois: `"meu-trackfw guard credential-hook --verbose"` + campo `timeout` extra
- Reconciliação: strings.Contains classificaria a entrada como guard e a sobrescreveria; igualdade exata não

### AC6 — Arms do D14 passaram a usar dirs pré-semeados com dados legados
- Antes: todos os 3 arms chamavam InjectCursorHooks em dir vazio (removeGuardFromFlatArrayInHooks nunca exercitado)
- Depois: cada arm semeia beforeShellExecution/afterShellExecution/preToolUse/postToolUse com formas legadas reais
- ArmA: adiciona asserção que formas legadas foram REMOVIDAS dos pares preToolUse/postToolUse também (não só dos shell events)
- ArmB: verifica formas legadas removidas de shell events; user hook sobrevive; pares sempre presentes
- ArmC: verifica formas legadas git removidas de shell events; cred presente

### AC7 (IdempotentFromLegacy) — semente expandida para todos os 4 pares + shell events
- Antes: apenas preToolUse/Read e postToolUse/Read semeados
- Depois: preToolUse/Read, preToolUse/Write, postToolUse/Read, postToolUse/Write + beforeShellExecution + afterShellExecution

### AC1 (ValidatorAfterMigration) — ValidateCredentialGuardHookResolvableForTest adicionado
- Adicionada Assertion 3: após migração, ValidateCredentialGuardHookResolvableForTest retorna 0 avisos
- Antes da migração haveria avisos de formas legadas; após, nenhum

### migrateDedupGuardInNestedArray — removida (dead code com bug D13)
- A função existia mas não era usada por nenhum sítio de produção
- Tinha bug: parava após o primeiro envelope com o matcher, uma duplicata em segundo envelope sobrevivia
- Substituída por comentário reservando o espaço para ML-1B (Claude/Codex/Gemini)

## Counter-arms em origin/main (evidência)

Configuração: `git archive origin/main` + cópia de `internal/guardforms/` e arquivos de teste WIP.

```
go test ./internal/generators/ -run TestCursorGuardMigrateDedup (origin/main tree):
  FAIL: UserHookSurvives
  FAIL: D14Arms/ArmA_NoGlobal
  FAIL: D14Arms/ArmB_GlobalCredInstalled
  FAIL: D14Arms/ArmC_GlobalGitBranchInstalled
  PASS: Idempotent (empty dir — comportamento não regressivo)
  FAIL: IdempotentFromLegacy

go test ./internal/validator/ -run TestCursorGuardMigrateDedup (origin/main tree):
  FAIL: ValidatorAfterMigration
  PASS: RegistryConcordance (propriedade bidirecional, correta em ambas as árvores)
```

## Gates finais

```
go build ./...       → rc=0
go test ./internal/generators/ ./internal/validator/ ./internal/pathguard/ -count=1
→ ok generators  58.421s | ok validator 31.101s | ok pathguard 0.888s

trackfw validate → 0 violations (163 warnings pré-existentes)
```

## Riscos remanescentes / ML-1B pendente

ML-1B (Claude, Codex, Gemini, Windsurf, Amazon Q, harness targets) ainda usa
migrateCursorSimpleCommand + migrateHookCommand em série. Não é regressão, mas acumula
duplicatas com múltiplas gerações legadas. ML-1B deve ser despachado em handoff separado.
