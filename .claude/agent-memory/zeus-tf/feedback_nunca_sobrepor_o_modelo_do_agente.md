---
name: nunca-sobrepor-o-modelo-do-agente
description: Nunca passar `model` no Agent tool — o frontmatter do agente decide, e o roteamento de modelos existe para economizar tokens
metadata:
  type: feedback
---

🔴 **NUNCA passe o parâmetro `model` no Agent tool.** O modelo vem do `model:` do frontmatter do
arquivo instalado em `~/.claude/agents/`, e essa escolha é do projeto, não minha.

```
apolo-tf · hades-tf · ares-tf · demais especialistas   model: claude-sonnet-4-6
zeus-tf (eu)                                            model: claude-opus-5-5
```

**Why:** o roteamento de modelos existe para **economizar tokens**. O parâmetro `model` do Agent tool
tem precedência sobre o frontmatter, então passá-lo **silenciosamente ignora a configuração do
projeto** — não há erro, não há aviso, e o custo só aparece na fatura.

Em 2026-09-26 eu passei `model: opus` em **doze despachos** de uma sessão só, sobrescrevendo Sonnet em
todos. Quando questionado, racionalizei com *"refutação degrada em modelo menor"* — hipótese que eu
**nunca medi**, e infalsificável do jeito que a usei, porque só observei o braço em que já tinha
trocado o modelo.

**O erro de método por trás:** o contrato de dispatch manda **ler o arquivo do agente** antes de
despachar, para obter o `subagent_type`. Eu li o `name:` e **não** o `model:`, que está na linha ao
lado. Régua estreita demais — o mesmo defeito que passei a campanha cobrando dos executores.

**How to apply:** ao chamar o Agent tool, passe `subagent_type` e **omita `model`**. Se um ML
específico parecer justificar modelo maior, **peça ao usuário com a razão** em vez de decidir sozinho
— a decisão é de custo, e custo é dele.

Relacionado: [[zeus-subagent-type]] (a outra metade do mesmo arquivo de agente, que eu já lia).
