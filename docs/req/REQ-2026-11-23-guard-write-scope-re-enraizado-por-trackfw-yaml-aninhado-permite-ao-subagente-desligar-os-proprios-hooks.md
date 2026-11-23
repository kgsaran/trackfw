---
status: Open
date: 2026-11-23
author: "zeus-tf (aberta a pedido do KG em 2026-11-23)"
adr: ""
roadmap: "docs/roadmaps/backlog/ROADMAP-2026-11-23-guard-write-scope-re-enraizado-por-trackfw-yaml-aninhado-permite-ao-subagente-desligar-os-proprios-hooks.md"
---

# REQ: Guard write-scope re-enraizado por trackfw.yaml aninhado permite ao subagente desligar os próprios hooks

> Date: 2026-11-23 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation
Achado F-5 do red team da REQ-2026-11-22 (`docs/seguranca/2026-11-22-red-team-onboarding.md`, seção F-5), anterior a
ela: o guard write-scope resolve a raiz do projeto pelo primeiro ancestral com `trackfw.yaml`
(`internal/guard/writescope.go`, `findProjectRoot`, ~linha 504). Um subagente pode:

1. escrever `.claude/trackfw.yaml`, que não está em `frozen_paths` → rc=0;
2. escrever `.claude/settings.json`: o guard agora enraíza em `.claude/`, onde o `trackfw.yaml` novo não tem
   `write_scope` → rc=0.

Em duas escritas, o subagente desliga os próprios hooks do Claude Code. O ADR-2026-10-23 (D3) documenta o
enraizamento pelo ancestral mais próximo, mas não declara este efeito.

## Acceptance Criteria
- [ ] Criar um `trackfw.yaml` aninhado não muda a raiz nem as regras que o guard write-scope aplica a um subagente
  (braço e contra-braço medidos no binário)
- [ ] Um `trackfw.yaml` novo fora da raiz do projeto, escrito por subagente, é tratado como caminho congelado ou
  bloqueado, decisão registrada em ADR
- [ ] Mesmo mecanismo varrido nos outros guards e nos comandos que resolvem a raiz pelo `trackfw.yaml` mais próximo
- [ ] `make quality` rc=0 e CI verde

## Negative scope
- Não muda `trackfw init` nem o leitor de config (REQ-2026-11-22).
- Não muda o guard de credencial nem o de git-branch, a não ser que a varredura ache o mesmo mecanismo neles.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/backlog/ROADMAP-2026-11-23-guard-write-scope-re-enraizado-por-trackfw-yaml-aninhado-permite-ao-subagente-desligar-os-proprios-hooks.md
