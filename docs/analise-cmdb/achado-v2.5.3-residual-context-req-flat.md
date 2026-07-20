# Achado v2.5.3 (residual, NÃO-bloqueante) — `context`/checks de REQ ainda flat

> **Origem:** revalidação no CMDB após o fix v2.5.3 (REQ indexing by_agent no traceid).
> **Autor:** 🌩️ Zeus — arquiteto do CMDB · **Data:** 2026-06-14
> **Destinatário:** agente/mantenedor do trackfw · **Severidade:** 🟡 não-bloqueante (cosmético + checks inertes).
> **Alvo sugerido:** v2.5.4 (oportunístico).

---

## ✅ O que foi corrigido (desbloqueia a migração do CMDB)

A coleta de REQ do **traceid** passou a honrar `roadmap_namespacing: by_agent`. Validado de campo:

- Par CORRETO (REQ+Roadmap mesmo `req_id`, mesmo estado, em `<agente>/<estado>/`) → **zero** traceid. ✅
- Estado divergente (REQ `done` × Roadmap `wip`) → `traceid_state_mismatch`. ✅
- No CMDB real, os falsos `traceid_orphan_roadmap` caíram de **15 → 8**, e surgiu `traceid_orphan_req` (4)
  — sinal de que as REQs passaram a ser indexadas e pareadas pelo traceid.

Com isso, o equivalente a R9/R10 funciona no layout by_agent — o ADR-039 §4 do CMDB **pode prosseguir**.

## 🟡 Resíduo — `context` e os checks não-traceid de REQ continuam flat

Mesmo com o traceid achando as REQs, **`trackfw context` ainda reporta `## REQs (0)`** — inclusive num
caso controlado onde o traceid claramente pareou a REQ. Ou seja, o fix entrou no **caminho do traceid**
(`resolveREQFiles`), mas **não** no contador do `context` nem nos demais checks de REQ.

Evidência (caso controlado by_agent, par perfeito):
```
## REQs (0)        ← context errado (a REQ existe e o traceid a achou)
## Roadmaps (1)
traceid: (nenhum)  ← traceid correto: pareou
```

Provavelmente afetados (usam `listDir(cfg.REQDir)` **não-recursivo**):
- `trackfw context` — contador/listagem de REQs.
- `validateREQsHaveADR` (REQ → ADR).
- `validateREQsHaveRoadmap` (REQ → Roadmap).
- `validateFrontmatterPresence` (parte de REQ).

Impacto: em projeto by_agent, esses checks ficam **inertes** (não acham REQs) — não geram ruído, mas
**não validam nada** do lado REQ; e o `context` engana ao mostrar 0 REQs.

## Correção sugerida

Aplicar a **mesma** resolução de diretórios usada agora no traceid (`resolveREQFiles`) a TODOS os
consumidores de REQ — `context`, `validateREQsHaveADR`, `validateREQsHaveRoadmap`,
`validateFrontmatterPresence` — substituindo o `listDir` flat por varredura consciente de namespacing.
Idealmente, centralizar num único helper reutilizado por todos (evita divergência futura entre caminhos).

## Prioridade

🟡 Não bloqueia a migração R9/R10 do CMDB (o traceid já cobre). Vale corrigir para (a) o `context` não
enganar e (b) os checks REQ→ADR/REQ→Roadmap realmente rodarem em projetos by_agent.
