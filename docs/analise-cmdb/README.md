# Análise CMDB — Feedback de campo do trackfw

> Origem: integração do **trackfw v2.1.1** no projeto **CMDB** (monorepo real em produção).
> Autor da análise: 🌩️ Zeus (arquiteto do CMDB) · Data: 2026-06-13
> Destinatário: agente/mantenedor do **trackfw**.

Esta pasta reúne uma análise comparativa entre o **`trackfw validate`** e o **gate de governança
interno do CMDB** (`scripts/validate-kanban-gate.mjs`, especificado no ADR-036 do CMDB). O CMDB é o
projeto que **inspirou** a metodologia do trackfw, então os dois validadores resolvem o **mesmo
problema** (governança `ADR → REQ → ROADMAP → backlog/wip/done`) por caminhos diferentes — o que os
torna um ótimo banco de provas para evoluir o trackfw.

## Conteúdo

- [`analise-comparativa-gates-governanca.md`](./analise-comparativa-gates-governanca.md) — matriz de
  funcionalidades (regra a regra), comparação de design, e **4 achados acionáveis** para o trackfw.
- [`evolucao-generica-baseline-legado.md`](./evolucao-generica-baseline-legado.md) — discussão pós-v2.3.0:
  como ser "genérico" para projetos legados **sem perder os dentes** — os dois eixos (convenção × rigor)
  e o padrão **baseline + catraca** (field mapping, grandfather por data/snapshot, severidade por regra).
- [`achados-v2.4.0-baseline-warnings-e-config.md`](./achados-v2.4.0-baseline-warnings-e-config.md) —
  validação de campo da v2.4.0: field mapping ✅ e severidade por regra ✅; **baseline ratcheia só
  `violations`, não `warnings`** (anula o caso de legado) + gotcha de aspas no parser de config.
- [`achado-upstream-id-rastreabilidade-e-json.md`](./achado-upstream-id-rastreabilidade-e-json.md) —
  pós-v2.4.1: 2 recursos de **rigor geral** sugeridos para o trackfw (não específicos do CMDB):
  **ID estável `req_id`** (pareamento bidirecional + estado + unicidade lógica) e **saída `--json`**.
- [`achados-v2.5.0-json-fields-e-docs.md`](./achados-v2.5.0-json-fields-e-docs.md) — validação da v2.5.0:
  `trace_id_field` (5 checks) ✅ completo; `--json` válido mas com **`rule`/`file` vazios** (só `message`)
  + `trace_id_field`/`rules.traceid_*` ausentes do `trackfw help`.
- [`achado-v2.5.1-traceid-nao-suporta-by-agent.md`](./achado-v2.5.1-traceid-nao-suporta-by-agent.md) —
  🔴 **bloqueante**: os checks `traceid_*` **não cobrem `roadmap_namespacing: by_agent`** (varrem só
  `rootDir/<estado>/`, não `rootDir/<agente>/<estado>/`) → 0 REQs indexadas no CMDB; `validate` dá falso
  exit 0. Bloqueou a migração R9/R10 (ADR-039 §4 do CMDB).
- [`achado-v2.5.2-req-indexing-by-agent-incompleto.md`](./achado-v2.5.2-req-indexing-by-agent-incompleto.md) —
  🔴 **ainda bloqueante**: o fix v2.5.2 corrigiu **Roadmaps** (116 indexados no CMDB) mas **REQs seguem
  (0)** — a coleta de REQ não honra by_agent. Gera `traceid_orphan_roadmap` falso em massa. Falta simetria
  na coleta de REQs (espelhar o fix do roadmap_dir). Isolado: não é o `ç`.
- [`achado-upstream-rules-req-configuraveis.md`](./achado-upstream-rules-req-configuraveis.md) — 🟡 rigor
  geral (opt-in): tornar `req_has_adr`/`req_has_roadmap`/`blocked_has_req` **severidades configuráveis**
  (`rules.*`) como as demais — hoje são sempre-erro (assimetria). Surgiu ao alinhar o CMDB ao strict (ADR-040).
- [`achado-v2.5.3-residual-context-req-flat.md`](./achado-v2.5.3-residual-context-req-flat.md) — ✅ v2.5.3
  corrigiu o **traceid REQ indexing** by_agent (par correto = 0; state_mismatch ok) → **migração
  desbloqueada**. 🟡 Residual não-bloqueante: `trackfw context` e os checks `validateREQsHave*` ainda
  usam `listDir` flat → `REQs (0)` no context + checks REQ→ADR/Roadmap inertes em by_agent.

## Status dos achados (atualizado 2026-06-13)

A **v2.3.0** já endereçou os achados técnicos prioritários da análise comparativa (validado de campo no CMDB):

| Achado | Status na v2.3.0 |
|---|---|
| #2 `adr_dirs` não-recursivo | ✅ corrigido (`walkADRFiles`/`findADRFile`; 59 ADRs detectados no CMDB) |
| #1 pareamento por substring | ✅ existência de alvo (`validateRefTargetsExist`); reciprocidade/estado por ID ainda em aberto |
| #3 stale por `mtime` | ✅ corrigido (`gitLastModifiedTime` via `git log`) |
| bônus | ✅ adotaram `validateFolderStatusCoherence` (pasta×status) e `validateFilenameUniqueness` |
| #4 conflito de convenção | ✅ v2.4.0 — `link_fields.*` (field mapping) |

### v2.4.0 (validada de campo)

| Recomendação | Status |
|---|---|
| Field mapping (`link_fields.*`, `acceptance_markers`) | ✅ funciona |
| Severidade por regra (`rules.*` = off/warning/error) | ✅ funciona |
| Baseline + ratchet (`trackfw baseline`) | ⚠️ parcial — só `violations`, não `warnings` |
| Gotcha: valor de regra entre aspas (`"off"`) quebra parsing | 🐛 aberto |

Próxima iteração sugerida (v2.4.1/v2.5): **ratchet de warnings no baseline** + **trim de aspas no
parser** (ou migrar para lib YAML). Detalhes em `achados-v2.4.0-baseline-warnings-e-config.md`.

### v2.4.1 / v2.5.0 (validadas de campo)

| Item | Status |
|---|---|
| v2.4.1 — ratchet de warnings no baseline | ✅ funciona (CMDB: 132 itens congelados → validate=0) |
| v2.4.1 — trim de aspas no parser | ✅ funciona |
| v2.5.0 — `trace_id_field` (5 checks bidirecionais R9/R10) | ✅ completo |
| v2.5.0 — `--json` | ✅ válido (stdout); `rule`/`file` vazios → ✅ corrigido na v2.5.1 |
| v2.5.0 — docs do `help` p/ chaves novas | 🐛 gap → ✅ corrigido na v2.5.1 |
| v2.5.1 — `--json` popula `rule`/`file` | ✅ validado (0 itens vazios) |
| v2.5.1 — `help` lista `trace_id_field` + `rules.traceid_*` | ✅ validado |

**Ciclo de feedback ENCERRADO (v2.1.1 → v2.5.1):** todos os achados levantados pelo CMDB foram
endereçados e validados de campo. O trackfw agora cobre os equivalentes de R9/R10 (`trace_id_field`) e
tem `--json` plenamente consumível por CI — satisfazendo o **gatilho de revisão do ADR-039 §4** (migração
de R9/R10 do `.mjs` para o trackfw, quando o CMDB priorizar).

## TL;DR para o agente do trackfw

Os dois gates são **majoritariamente complementares** (sobreposição real de apenas 3 regras). O gate
interno do CMDB é **mais rigoroso na integridade REQ↔ROADMAP** (pareamento por `req_id` com
reciprocidade, pasta×status, unicidade lógica, grandfather por data, evidência em `done`). O trackfw é
**mais amplo na cadeia** (cobre o elo **ADR**, WIP limit, critérios de aceite, gating por ADR-Draft) e
traz **ecossistema** (`status`, `metrics`, `serve`, `sync`, instaladores de agents).

**2 achados que valem priorização no trackfw:**

1. **Pareamento por substring é frágil.** `validate` casa `REQ:`/`ADR:`/`Roadmap:` por
   `strings.Contains`, sem verificar **existência do alvo**, **reciprocidade** do vínculo nem
   **compatibilidade de estado**. O CMDB usa uma chave estável (`req_id`) com verificação bidirecional —
   referência de design que elimina falsos negativos (um link que aponta para nada "passa").

2. **`adr_dirs` é lido de forma não-recursiva** (`listDir`/`glob *.md`). Projetos que organizam ADRs em
   subpastas (ex.: `docs/adr/<agente>/<estado>/`) ficam **invisíveis** ao trackfw — os checks de ADR
   (REQ→ADR, ADR órfão, ADR-Draft gating, frontmatter de ADR) viram **no-ops silenciosos**. Aconteceu no
   CMDB após uma reorganização legítima.

Detalhes, reprodução e recomendações no documento principal.
