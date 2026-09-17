---
type: bug-root-cause
date: 2026-09-17
req: REQ-2026-09-17-leniencia-sem-prazo-e-eterna-e-a-severidade-por-regra-vem-do-arquivo-que-o-pr-edita.md
ml: ML-1B
---

# Dois sistemas de defaults divergentes: `config.defaultProjectConfig().Rules` vs `ruleDefaults` / `credentialGuardDefaultSeverity`

## Raiz do problema

O pacote `config` tem seus próprios defaults embutidos em `defaultProjectConfig().Rules`:

```
stale_wip:    "warning"
adr_orphan:   "warning"
folder_status: "warning"
```

Esses defaults são **injetados automaticamente** em `config.Load().Rules` mas **não existem** em `ruleDefaults` (definidos em `validator.go`) nem são retornados por `credentialGuardDefaultSeverity(name)` — que retorna "error" para qualquer nome não listado.

## Consequência medida em ML-1B

O código de weakening-check em `originAnchorRefUnreadable` compara:

```go
diskSev := cfg.Rules[ruleName]           // "warning" — vem do config default
defSev  := credentialGuardDefaultSeverity(ruleName)  // "error" — não conhece o config default
```

Para regras de governança (`stale_wip`, `adr_orphan`, `folder_status`):
- `cfg.Rules["stale_wip"] = "warning"` (config default)
- `credentialGuardDefaultSeverity("stale_wip") = "error"` (não está em ruleDefaults)
- Resultado: falso-positivo de weakening — violação emitida mesmo que o usuário não tenha escrito nada no `rules:` do seu `trackfw.yaml`

## Fix aplicado

Usar `config.ParseRulesFromContent(string(rawContent))` em vez de `config.Load().Rules`.

`ParseRulesFromContent` retorna SOMENTE as regras que o usuário escreveu explicitamente no bloco `rules:` do arquivo `trackfw.yaml` em disco — sem injetar defaults do pacote config. Assim, `stale_wip`, `adr_orphan` e `folder_status` não aparecem na iteração se o usuário não os declarou, eliminando o falso-positivo.

## Onde está o código correto

`internal/validator/validator.go` — bloco `case originAnchorRefUnreadable:` em `ValidateUnfiltered()` e `validateUnfilteredTagged()`.

## Sintoma que leva a descoberta

Teste `TestOriginMainAnchor_RefUnreadable_SemEnfraquecimento_NaoReprova` falha com violações de `stale_wip`, `adr_orphan`, `folder_status` mesmo com fixture sem `rules:` explícito — porque `config.Load()` popula esses campos com os defaults do pacote.

## Regra de ouro

**Sempre que precisar saber "o que o usuário escreveu no trackfw.yaml"** (não "qual é a configuração efetiva"), usar `config.ParseRulesFromContent()`. Usar `config.Load().Rules` quando quiser a configuração efetiva (defaults + overrides).
