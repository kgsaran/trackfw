---
name: REQ-2026-06-14-rules-req-configuraveis
title: "v2.6.0 — Feat: req_has_adr / req_has_roadmap / blocked_has_req configuráveis via applyRule"
status: Open
adr: —
roadmap: docs/roadmaps/claude/wip/v2.6.0-rules-req-configuraveis-2026-06-14.md
created: 2026-06-14
author: zeus
---

# REQ — v2.6.0 Feat: tornar req_has_adr / req_has_roadmap / blocked_has_req configuráveis

## Contexto

Achado upstream reportado pelo arquiteto do CMDB após alinhamento ao ADR-040.
Arquivo de origem: `docs/analise-cmdb/achado-upstream-rules-req-configuraveis.md`

As três regras `validateREQsHaveADR`, `validateREQsHaveRoadmap` e `validateBlockedHasREQ`
são **sempre-erro** (append direto em `violations`, sem `applyRule`), diferente de todas
as outras regras que têm `rules.<nome>` configurável (off/warning/error).

---

## Feature

Rotear as três regras por `applyRule`/`applyRuleTagged` como as demais, com chaves de
config e defaults preservando o comportamento atual (`"error"`):

```yaml
rules:
  req_has_adr:      "error"   # default = comportamento atual
  req_has_roadmap:  "error"
  blocked_has_req:  "error"
```

### Go (`internal/validator/validator.go`)

Em `validateUnfiltered` (~linhas 306–322): substituir 3 blocos `violations = append(...)` por:
```go
applyRule("req_has_adr",     reqViolations,      &violations, &warnings)
applyRule("blocked_has_req", blockedViolations,  &violations, &warnings)
applyRule("req_has_roadmap", reqRoadmapViolations, &violations, &warnings)
```

Em `validateUnfilteredTagged` (~linhas 441–463): substituir 3 loops `for _, m := range ...` por:
```go
applyRuleTagged("req_has_adr",     reqViolations,      &violations, &warnings)
applyRuleTagged("blocked_has_req", blockedViolations,  &violations, &warnings)
applyRuleTagged("req_has_roadmap", reqRoadmapViolations, &violations, &warnings)
```

### Node.js (`npm/src/validator/index.js`)

Em `validate` (~linhas 823–825): substituir loops `for (const msg of ...)` por:
```js
applyRule('req_has_adr',     validateREQsHaveADR(),     violations, warnings)
applyRule('blocked_has_req', validateBlockedHasREQ(),   violations, warnings)
applyRule('req_has_roadmap', validateREQsHaveRoadmap(), violations, warnings)
```

### Python (`pypi/trackfw/validator.py`)

Em `validate` (~linhas 892–894): substituir `violations += _enrich_items(...)` por:
```python
_apply_rule("req_has_adr",     validate_reqs_have_adr(cfg),      violations, warnings, cfg)
_apply_rule("blocked_has_req", validate_blocked_has_req(cfg),    violations, warnings, cfg)
_apply_rule("req_has_roadmap", validate_reqs_have_roadmap(cfg),  violations, warnings, cfg)
```

---

## Critérios de aceite

- [ ] `rules.req_has_adr: "warning"` move violations para warnings nos 3 CLIs
- [ ] `rules.req_has_adr: "off"` silencia a regra nos 3 CLIs
- [ ] Default sem config (`"error"`) preserva comportamento atual
- [ ] `rules.blocked_has_req` e `rules.req_has_roadmap` idem
- [ ] Em `--json`, as violations/warnings carregam `"rule": "req_has_adr"` etc. (Go)
- [ ] Testes nos 3 CLIs cobrindo `warning` e `off` para cada regra
- [ ] Paridade nos 3 CLIs (Go · Node.js · Python)

## Não está no escopo

- Outras regras ainda sem `applyRule` (`validateFrontmatterPresence` etc.)
- Mudança no schema de config além das 3 novas chaves
