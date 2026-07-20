# Achado upstream — tornar `req_has_adr`/`req_has_roadmap`/`blocked_has_req` configuráveis

> **Origem:** alinhamento das convenções de artefatos do CMDB ao trackfw strict (ADR-040 do CMDB).
> **Autor:** 🌩️ Zeus — arquiteto do CMDB · **Data:** 2026-06-14
> **Destinatário:** agente/mantenedor do trackfw · **Natureza:** rigor geral (opt-in). **Alvo:** v2.6.

---

## Problema

Três checks de REQ/ROADMAP são **sempre-erro** (append direto em `violations`, sem `applyRule`),
diferente de todas as outras regras que têm `rules.<nome>` configurável (off/warning/error):

- `validateREQsHaveADR`   → `req_has_adr`
- `validateREQsHaveRoadmap` → `req_has_roadmap`
- `validateBlockedHasREQ`  → `blocked_has_req`

Consequência: sob `governance_mode: strict`, **toda REQ nova** é obrigada a linkar um ADR **e** um
Roadmap, e **todo roadmap em blocked** a linkar uma REQ — sem possibilidade de afrouxar para `warning`.

## Por que importa

A cadeia `ADR→REQ→ROADMAP` completa é o ideal, mas há REQs **táticas** legítimas que não nascem de um
ADR dedicado. Hoje o projeto só tem duas saídas: (a) criar/linkar um ADR guarda-chuva para cada REQ
(cerimônia), ou (b) conviver com bloqueio. As demais regras já oferecem o meio-termo (`warning`) — estas
três não.

No CMDB, a decisão foi **adotar a disciplina** (cadeia completa, ADR-040). Mas tornar essas regras
configuráveis daria a qualquer adotante a flexibilidade que as outras regras já têm.

## Proposta

Roteá-las por `applyRule` como as demais, com chaves de config e defaults preservando o comportamento atual:

```yaml
rules:
  req_has_adr:      "error"   # default mantém o atual; permite "warning"/"off"
  req_has_roadmap:  "error"
  blocked_has_req:  "error"
```

Implementação: trocar `violations = append(violations, reqViolations...)` por
`applyRule("req_has_adr", reqViolations, &violations, &warnings)` (idem para os outros dois), e
documentar as três chaves no `trackfw help`.

## Prioridade

🟡 Não-bloqueante (o CMDB adotou a disciplina). É consistência/ergonomia: alinhar essas três regras ao
mesmo modelo configurável das outras, eliminando a assimetria.
