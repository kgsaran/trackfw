---
name: REQ-2026-06-13-validate-json-output
title: "v2.5 — validate --json: saída estruturada para integração CI/CD"
status: Open
adr: —
roadmap: docs/roadmaps/claude/wip/v2.5-discovery-json-traceid-2026-06-13.md
created: 2026-06-13
author: zeus
---

# REQ — validate --json

## Contexto

`trackfw validate` emite apenas texto legível por humano. Pipelines de CI, dashboards de
qualidade e anotações de PR precisam de saída estruturada e estável.

Origem: `docs/analise-cmdb/achado-upstream-id-rastreabilidade-e-json.md` (Upstream 2).
O gate interno do CMDB já oferece `--json` nesse formato como referência de implementação.

---

## Proposta

Flag `--json` que serializa o resultado completo do `validate`:

```json
{
  "summary": { "violations": 0, "warnings": 60, "mode": "lenient", "exit_code": 0 },
  "violations": [ { "rule": "wip_has_req", "file": "docs/roadmaps/wip/foo.md", "message": "..." } ],
  "warnings":   [ { "rule": "adr_orphan",  "file": "docs/adr/ADR-001.md",      "message": "..." } ]
}
```

Campos por item: `rule` (nome da regra de `rules.*`), `file`, `message`.
Saída em stdout; erros internos em stderr.
Exit code inalterado (comportamento atual mantido).

---

## Critérios de aceite

- [ ] `trackfw validate --json` emite JSON válido em stdout
- [ ] `summary.mode` reflete `lenient` ou `strict`
- [ ] Cada item de `violations` e `warnings` tem `rule`, `file`, `message`
- [ ] Exit code é o mesmo que sem `--json`
- [ ] Sem `--json`: saída texto inalterada (retrocompatível)
- [ ] Paridade nos 3 CLIs (Go · Node.js · Python)
- [ ] Testes novos cobrindo o formato JSON nos 3 CLIs

---

## Não está no escopo

- Flag `--format` genérica ou outros formatos (CSV, YAML)
- Streaming de resultados (saída única ao final, como hoje)
