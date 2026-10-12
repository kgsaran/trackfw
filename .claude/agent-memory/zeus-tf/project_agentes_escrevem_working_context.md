---
name: agentes-escrevem-working-context
description: Subagentes (apolo/hefesto/hades) escrevem docs/agents-working-context.md mesmo proibidos no handoff — 5 de 9 em 2026-10-12; a regra do próprio agente vence o handoff
metadata:
  type: project
---

Em 2026-10-12, 5 de 9 subagentes escreveram em `docs/agents-working-context.md` apesar de o handoff
proibir em 🔴 (alguns até depois de eu citar o defeito no prompt). Só os que receberam "a auditoria
conta como defeito" + o motivo obedeceram de forma consistente.

**Why:** o arquivo de agente gerado (`<slug>-tf.md`) manda registrar início/fim no working context;
essa instrução de sistema pesa mais que a proibição no prompt. Conteúdo vinha correto — custo é
ruído no diff e commit de arquivo inesperado.

**How to apply:** não trate como insubordinação na auditoria; confira o conteúdo e commite. A
correção de origem é no gerador do agente (trocar "atualize o working context" por "o arquiteto
atualiza; você reporta") — candidata a REQ, mesma família de [[fronteira-de-escrita-dos-auditores]].
