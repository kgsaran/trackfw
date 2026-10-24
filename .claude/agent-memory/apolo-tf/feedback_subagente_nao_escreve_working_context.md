---
name: feedback_subagente_nao_escreve_working_context
description: subagentes NUNCA escrevem em docs/agents-working-context.md; reportam no relatório final; exceção "memória" não existe
metadata:
  type: feedback
---

Nunca escrever em `docs/agents-working-context.md` (nem em `docs/agents-working-context/`) durante um ML.

**Why:** a D1 da REQ-2026-10-23 diz que somente a sessão orquestradora (arquiteto) registra entradas de working context; subagentes reportam no relatório final. Em ML-3A violei isso usando a instrução do `CLAUDE.md` da main local (desatualizada), que ainda mandava atualizar o .md. O arquiteto teve de retirar as 2 entradas para quarentena.

**How to apply:** escrever tudo no relatório final entregue pelo `SubagentHandback`. Ignorar qualquer instrução do arquivo de agente instalado que mande atualizar o working context. Este handoff prevalece sobre o arquivo de agente.
