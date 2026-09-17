# origin/main anchor: var de pacote `currentOriginMain` fica stale entre testes (2026-09-17)

## Problema

`currentOriginMain` é uma `var` de pacote em `internal/validator/validator_credential_guard_integrity.go`.
Ela é carregada em `ValidateUnfiltered()` e `validateUnfilteredTagged()` (no início de cada chamada).
Testes que chamam `ruleSeverity()` **diretamente** (sem passar por `Validate*()`), leem o valor que
a execução de teste anterior deixou.

**Sintoma:** dois testes que chamavam `ruleSeverity()` direto falharam intermitentemente após a
adição de `initOriginMain()` em outros testes do mesmo pacote. O valor residual de
`originAnchorOK` de um teste anterior fazia `ruleSeverity("wip_limit")` retornar `"error"` (o
padrão built-in) em vez de `"warning"` (do disco), porque `currentOriginMain.rules["wip_limit"]`
estava ausente → `credentialGuardDefaultSeverity("wip_limit")` = "error" → stricter-wins →
"error".

## Solução

Dois padrões, um para cada caso:

### Teste que usa `ValidateUnfiltered()` e configura `initOriginMain()`

Adicionar:
```go
t.Cleanup(func() { currentOriginMain = originMainAnchor{} })
```

Isso reseta a var para o zero value (`state == originAnchorNotSet`) ao sair do subteste,
evitando que o estado `originAnchorOK` vaze para o próximo teste.

### Teste que chama `ruleSeverity()` diretamente

Adicionar, logo antes da chamada a `ruleSeverity()`:
```go
currentOriginMain = loadOriginMainAnchor()
```

Isso re-avalia o estado para o diretório atual, garantindo que `ruleSeverity()` leia o
estado correto e não o stale de outro teste.

## Contexto

Padrão introduzido no ML-1A do ROADMAP-2026-09-17-leniencia-sem-prazo-e-eterna-e-a-severidade-por-regra-vem-do-arquivo-que-o-pr-edita.
Regras afetadas: `TestRuleSeverity_ZeroDeltaParaRegrasNaoGuard` e `TestCredentialGuardRuleSeverity_SemHead_CaiNoDisco`.

## Regra geral

Toda var de pacote que guarda estado entre chamadas de `Validate*()` é um candidato a stale state
em testes. O padrão de cleanup/reload deve ser aplicado a qualquer futuro estado de pacote que
siga o mesmo modelo (ex.: se um segundo tipo de anchor for adicionado).
