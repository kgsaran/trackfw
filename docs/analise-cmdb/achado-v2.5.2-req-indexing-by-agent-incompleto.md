# Achado v2.5.2 (bloqueante) — fix by_agent **parcial**: REQs não são indexadas

> **Origem:** revalidação no CMDB após o fix v2.5.2 do achado anterior.
> **Autor:** 🌩️ Zeus — arquiteto do CMDB · **Data:** 2026-06-14
> **Destinatário:** agente/mantenedor do trackfw · **Severidade:** 🔴 ainda bloqueante.
> **Pré-requisito:** [`achado-v2.5.1-traceid-nao-suporta-by-agent.md`](./achado-v2.5.1-traceid-nao-suporta-by-agent.md)
> **Alvo sugerido:** v2.5.3.

---

## O que melhorou na v2.5.2 ✅

O fix `roadmap_namespacing: by_agent` + salvaguarda zero-entradas funciona para **Roadmaps**:

- No CMDB real: `trackfw context` agora indexa **Roadmaps (116)** (era 0 na v2.5.1).
- Salvaguarda validada: com `trace_id_field` setado e 0 entradas, emite
  `⚠ trace_id_field is set but no REQ/Roadmap entries were indexed — check req_dir, roadmap_dir and roadmap_namespacing`. 👏

## O que ainda quebra 🔴 — REQs continuam **(0)**

No CMDB real (`trackfw context` com `trace_id_field` habilitado):

```
## ADRs (0)
## REQs (0)          ← ainda zero
## Roadmaps (116)    ← corrigido
```

Consequência: como **nenhuma REQ é indexada**, todo Roadmap que tem `req_id` aparece como
`traceid_orphan_roadmap` — **falsos positivos em massa** (15 no CMDB), e os checks que dependem da REQ
(`orphan_req`, `duplicate_req`, `state_mismatch`) ficam sem base.

## Causa raiz (isolada — não é encoding)

A coleta de **REQs** não honra `roadmap_namespacing: by_agent` — ela não percorre
`req_dir/<agente>/<estado>/`. Só a coleta de **Roadmaps** recebeu o tratamento by_agent.

Reprodução controlada (REQ em `<agente>/wip/`, Roadmap pareado por `req_id`):

```bash
# A) req_dir ASCII (docs/req) + by_agent + REQ em docs/req/claude/wip/req.md
→ trackfw context: ## REQs (0)     ← REQ NÃO indexada
   traceid: traceid_orphan_roadmap  (falso positivo — o par RID-1 existe)

# B) req_dir NÃO-ASCII (docs/requisições) — idêntico ao CMDB
→ trackfw context: ## REQs (0)     ← idem
```

A=B ⇒ **descartado o fator não-ASCII** (`ç`). A causa é puramente o **namespacing by_agent não aplicado
à coleta de REQs**.

### Observação adicional (modelo de REQ)

Vale conferir se o problema é mais amplo que o traceid: outras checagens de REQ
(`validateREQsHaveADR`, `validateREQsHaveRoadmap`) usam `listDir(cfg.REQDir)` **não-recursivo** — ou
seja, esperam REQs como arquivos **diretamente** em `req_dir`. Em projetos que organizam REQs por
`<agente>/<estado>/` (simétrico aos roadmaps, como o CMDB exige no ADR-036), **todas** as checagens de
REQ ficam inertes, não só o traceid.

## Correção sugerida

1. **Coleta de REQ consciente de namespacing:** `collectTraceIdEntries` para o `req_dir` deve percorrer
   `req_dir/<agente>/<estado>/` quando `roadmap_namespacing == by_agent` — **espelhando** exatamente o
   que já foi feito para o roadmap_dir na v2.5.2 (reusar a mesma resolução de diretórios / `resolveWIPDirs`).
2. **Estender às demais checagens de REQ** (`validateREQsHaveADR`, `validateREQsHaveRoadmap`,
   `validateFrontmatterPresence` para REQ, etc.): trocar `listDir` flat por uma varredura recursiva
   consciente de namespacing, para que REQs nested sejam encontradas.
3. (Opcional, mas recomendado) Reforçar a **salvaguarda**: hoje ela só dispara quando **ambos** os lados
   dão 0. Considerar disparar também quando **um** dos lados (REQ **ou** Roadmap) dá 0 mas o outro não —
   exatamente o caso atual do CMDB (Roadmaps 116, REQs 0), que passou sem aviso de salvaguarda.

## Impacto

🔴 **Ainda bloqueia** a migração R9/R10 do CMDB (ADR-039 §4). Migrar agora produziria falsos
`traceid_orphan_roadmap` em massa e **não** validaria o pareamento real (REQs invisíveis). O roadmap do
CMDB permanece `blocked` até a coleta de REQs honrar by_agent.

> Resumo: a v2.5.2 acertou metade (Roadmaps) — falta a **simetria na coleta de REQs**.
