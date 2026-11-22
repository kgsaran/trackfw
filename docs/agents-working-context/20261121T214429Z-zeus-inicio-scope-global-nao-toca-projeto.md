# Zeus — início da REQ-2026-11-17: `--scope global` não toca o projeto

- **Agente:** zeus-tf (trackfw_architect)
- **Decisão do KG (2026-11-21):** com `--scope global`, `agents install` e `skills install` não escrevem nada no cwd (nem regras nem hooks) e imprimem a dica para o projeto; `--scope project` não muda.
- **Plano:** ADR → REQ atualizada → roadmap em wip → branch `fix/` → Wave 0 (hades) → Wave 1 (apolo) → Wave 2 (hefesto + red team hades) → make quality + CI.
- **Sítio conhecido:** `internal/commands/integrations_flags.go:277-289`. A varredura de mesma causa é o AC3 da REQ.
