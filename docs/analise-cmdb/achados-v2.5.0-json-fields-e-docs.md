# Achados da v2.5.0 — `--json` com `rule`/`file` vazios + gap de docs

> **Origem:** validação de campo da v2.5.0 no contexto do CMDB.
> **Autor:** 🌩️ Zeus — arquiteto do CMDB · **Data:** 2026-06-13
> **Destinatário:** agente/mantenedor do trackfw · **Alvo sugerido:** v2.5.1.
> **Contexto:** a v2.5.0 implementou os 2 itens de
> [`achado-upstream-id-rastreabilidade-e-json.md`](./achado-upstream-id-rastreabilidade-e-json.md):
> ID estável `req_id` (bidirecional) e saída `--json`.

---

## Resumo da validação

| Upstream | Status | Nota |
|---|---|---|
| ID estável `req_id` (`trace_id_field`) — 5 checks | ✅ **completo** | validado de campo (ver §1) |
| Saída `--json` | ✅ funciona, ⚠️ **incompleta** | `rule`/`file` vêm vazios (Achado 1) |
| Documentação das chaves novas | 🐛 gap | `trace_id_field`/`rules.traceid_*` ausentes do `trackfw help` (Achado 2) |

---

## ✅ O que está ótimo — `trace_id_field` (R9+R10 equivalentes)

`internal/validator/validator_traceid.go` — opt-in via `trace_id_field` (default `""` = desligado),
conectado ao `Validate()`. As **5 verificações** foram validadas empiricamente, todas disparam:

- `traceid_duplicate_req` — mesmo `req_id` em >1 REQ.
- `traceid_duplicate_roadmap` — mesmo `req_id` em >1 Roadmap.
- `traceid_orphan_req` — REQ com `req_id` sem Roadmap correspondente.
- `traceid_orphan_roadmap` — Roadmap com `req_id` sem REQ correspondente.
- `traceid_state_mismatch` — REQ e Roadmap com mesmo `req_id` em estados divergentes.

Nuance correta de design: `state_mismatch` deriva o estado da **subpasta** (`wip`/`done`/...), então só
compara quando ambos os lados residem em pastas de estado. Comportamento adequado.

Nada a corrigir aqui — atende plenamente o equivalente a R9/R10 do gate interno do CMDB.

---

## Achado 1 (médio) — `--json` não popula `rule` nem `file`

### Sintoma

A saída `--json` é **JSON válido em STDOUT** (e o resumo humano vai para STDERR — separação correta ✅).
Porém cada item traz `rule` e `file` **vazios**; só `message` é preenchido:

```json
{
  "summary": { "violations": 2, "warnings": 0, "mode": "strict", "exit_code": 1 },
  "violations": [
    { "rule": "", "file": "", "message": "roadmap \"rm.md\" is in wip but has no linked REQ" },
    { "rule": "", "file": "", "message": "roadmap \"rm.md\" is in wip but has no acceptance criteria block" }
  ],
  "warnings": []
}
```

### Causa raiz (provável)

As funções de validação retornam `[]string` (mensagens cruas); o nome da regra (`wip_has_req`,
`adr_orphan`, `traceid_*`, ...) e o arquivo afetado existem apenas **embutidos no texto** da mensagem,
não como campos. Na serialização JSON, os slots `rule`/`file` são preenchidos com `""`.

### Por que importa

- **Gate de CI puro** (falhar/passar): `exit_code` + `message` já bastam — OK.
- **Dashboards / anotações de PR / métricas por regra**: precisam de `rule` (para agrupar/medir) e
  `file` (para anotar `path:line`). Com ambos vazios, o consumidor teria que **re-parsear a mensagem por
  regex** — o que anula o ganho do JSON estruturado.

### Referência

O `--json` do gate interno do CMDB (`scripts/validate-kanban-gate.mjs`) já emite por item
`{ rule, file, message }` com `rule`/`file` preenchidos — pode servir de espelho de formato.

### Correção sugerida

Propagar `rule` e `file` desde a origem da violação. Duas formas:

1. **Estrutural (recomendada):** as validações passam a retornar registros
   `{ rule, file, message }` em vez de `[]string`; `applyRule(ruleName, ...)` já conhece o `ruleName` —
   basta carregá-lo no registro, e cada checagem já conhece o caminho do arquivo que inspecionou.
2. **Mínima (paliativa):** preencher ao menos `rule` no momento do `applyRule` (o nome da regra está
   disponível ali) e extrair `file` do path já manipulado em cada checagem.

Manter o texto de `message` como está (compatível com a saída humana).

---

## Achado 2 (menor) — chaves novas ausentes do `trackfw help`

`trackfw help` (tabela de chaves do `trackfw.yaml`) **não lista**:

- `trace_id_field` (string; default `""` = desligado).
- as severidades `rules.traceid_duplicate_req`, `rules.traceid_duplicate_roadmap`,
  `rules.traceid_orphan_req`, `rules.traceid_orphan_roadmap`, `rules.traceid_state_mismatch`.

Quem ativa `trace_id_field` não descobre as severidades configuráveis pela documentação embutida.
**Sugestão:** adicionar essas linhas à tabela do `help` (e ao wizard `configure`, se aplicável).

---

## Prioridade sugerida

1. **Achado 1** (`--json` popular `rule`/`file`) — habilita o uso real do JSON em CI/dashboards.
   Sem isso, o recurso fica meio-caminho.
2. **Achado 2** (docs do `help`) — trivial, melhora descoberta do `trace_id_field`.

> Veredito geral da v2.5.0: **muito boa** — `trace_id_field` entregue e validado (cobre R9/R10), `--json`
> com estrutura e separação de streams corretas. Falta só **preencher `rule`/`file`** para o JSON ser
> plenamente consumível por automação.

---

## Impacto no CMDB (ADR-039 §4)

A v2.5.0 **destrava o gatilho de revisão** da coexistência: com `trace_id_field` pronto, R9/R10 podem
migrar do `.mjs` para o trackfw. **Recomendação ao CMDB:** aguardar o fix do Achado 1 antes de depender
do `--json` para anotações; o `trace_id_field` já pode ser adotado quando o gatilho for acionado.
