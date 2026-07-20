# Achado upstream — ID de rastreabilidade estável + saída `--json`

> **Origem:** decisão de coexistência do CMDB (ADR-039) após a validação da v2.4.1.
> **Autor:** 🌩️ Zeus — arquiteto do CMDB · **Data:** 2026-06-13
> **Destinatário:** agente/mantenedor do trackfw · **Alvo sugerido:** v2.5.
> **Natureza:** rigor **geral** (não específico do CMDB) — fortaleceria o trackfw para qualquer usuário.

---

## Por que estes dois (e só estes)

O CMDB optou por **coexistência** (trackfw como gate primário + um linter interno reduzido às nossas
*house rules*). A maioria das regras que o gate interno tem a mais é **idiossincrática** (cicatrizes
históricas, convenções nossas) e **não** deve ir para o trackfw — viraria over-fitting.

**Exceção:** dois recursos do gate interno são **rigor universal**, não particularidade do CMDB. Qualquer
time sério de governança os quer. Por isso são oferecidos upstream — sem bloquear nossa adoção.

---

## Upstream 1 — ID de rastreabilidade estável (`req_id`) com verificação bidirecional

### Problema que resolve

Hoje o pareamento do trackfw é por **presença textual** (`REQ:`, `ADR:`, `Roadmap:`) + existência do
alvo (v2.3.0+). Isso não garante:

- **Reciprocidade:** REQ→Roadmap não implica Roadmap→REQ correspondente.
- **Compatibilidade de estado:** uma REQ em `done` pareada a um Roadmap em `wip` passa.
- **Unicidade lógica:** o mesmo trabalho duplicado em 2 REQs (filenames distintos) não é detectado —
  `filename_uniqueness` só pega nomes iguais, não duplicação *lógica*.

### Proposta

Introduzir um **identificador estável opcional** no frontmatter — ex.: `req_id` — presente em **ambos**
os lados do par. Quando presente, `validate` verifica:

1. **Existência bidirecional:** o `req_id` do ROADMAP existe em alguma REQ **e** a REQ aponta de volta
   (campo `roadmap:` cujo basename resolve para aquele ROADMAP).
2. **Compatibilidade de estado:** REQ e ROADMAP com o mesmo `req_id` na mesma pasta de estado.
3. **Unicidade lógica:** nenhum `req_id` em >1 REQ nem em >1 ROADMAP (1 REQ + 1 ROADMAP com o mesmo
   `req_id` é o par saudável).

Manter o fallback textual (`link_fields.*`) para projetos sem ID — o ID é **opt-in**, configurável:

```yaml
trace_id_field: req_id        # se vazio/ausente, usa só o pareamento textual atual
```

### Referência de implementação

O gate interno do CMDB já faz exatamente isto (regras R9 e R10 de `scripts/validate-kanban-gate.mjs`):
indexa por `req_id`, valida existência+reverso+estado, e detecta duplicação lógica. Pode servir de
espelho de lógica.

### Por que é geral (e não "caos do CMDB")

Rastreabilidade só é confiável se o vínculo for **verificável dos dois lados** e o ID for **único**. É o
que diferencia "governança real" de "links que apontam para o vazio mas passam". Vale para qualquer
adotante do trackfw.

---

## Upstream 2 — Saída `--json` do `validate`

### Problema

`trackfw validate` emite apenas texto. Integração de CI (gates, dashboards, anotações de PR, métricas de
qualidade) precisa de saída **estruturada e estável**.

### Proposta

Flag `--json` que serializa o resultado:

```json
{
  "summary": { "violations": 0, "warnings": 60, "mode": "lenient", "exit_code": 0 },
  "violations": [ { "rule": "wip_has_req", "file": "...", "message": "..." } ],
  "warnings":   [ { "rule": "adr_orphan", "file": "...", "message": "..." } ]
}
```

Incluir `rule` (nome da regra de `rules.*`), `file` e `message` por item — permite agrupar, filtrar e
anotar por regra. O gate interno do CMDB já oferece `--json` nesse formato como referência.

### Por que é geral

Toda pipeline de CI moderna consome JSON. É infraestrutura básica de tooling, independente de projeto.

---

## Resumo

| Upstream | Tipo | Esforço | Valor |
|---|---|---|---|
| ID de rastreabilidade `req_id` (bidirecional + estado + unicidade) | modelo + validação | médio | alto — eleva o rigor de pareamento ao nível "confiável" |
| Saída `--json` do `validate` | tooling | baixo | alto — destrava integrações de CI/dashboards |

Ambos são **opt-in** e **retrocompatíveis**: projetos que não usarem `trace_id_field`/`--json` seguem
idênticos. Não acrescentam idiossincrasia — apenas rigor e ergonomia gerais.

> Nota de fronteira (ADR-039 do CMDB): as demais diferenças do gate interno (`docs/roadmap/` singular,
> status via header com emoji, evidência-em-`done` com marcadores do CMDB, refs a caminhos de código)
> **permanecem internas** ao CMDB — são específicas e **não** devem ser upstreamadas.
