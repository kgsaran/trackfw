# Zeus — início da REQ-2026-11-23: write-scope re-enraizado por trackfw.yaml aninhado

- **Agente:** zeus-tf (trackfw_architect)
- **Branch:** fix/guard-write-scope-re-enraizado-trackfw-yaml-aninhado
- **Origem:** F-5 do red team da REQ-2026-11-22. Um subagente cria `.claude/trackfw.yaml` e, com isso, escreve em `.claude/settings.json` sob uma raiz sem regras.
- **Contexto:** o ADR-2026-10-23 já declara que a escrita por Bash escapa da D3 e da D4; a Wave 0 mede o que esta REQ fecha diante disso.
- **Plano:**
  - Wave 0 (hades): reprodução, varredura dos sítios que resolvem a raiz pelo `trackfw.yaml` mais próximo, e desenhos candidatos.
  - ADR.
  - Wave 1 (apolo), Wave 2 (revisão e red team), fechamento.
