# zeus — início da #618: marcador do guard fora de hook (2026-10-25)

- **Começou:** o KG priorizou a #618 na frente da fila. Criei a REQ-2026-10-25, a D16 no ADR-2026-10-04 e o roadmap com as Waves 0–3.
- **Causa lida:** `collectCommandsWithMarker` casa em qualquer string, e 12 chamadores passam o arquivo inteiro.
- **Próximo:** a Wave 0 com o hades-tf: campo de comando por CLI, matriz de chamadores e medição de fail-open.
