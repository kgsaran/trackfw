---
name: verbosidade-das-respostas
description: Respostas ao KG devem ser curtas por padrão; detalhe é sob demanda ou reservado a bloqueio, decisão e erro próprio
metadata:
  type: feedback
---

Responder **curto por padrão**. Detalhe extenso só quando houver bloqueio, decisão pendente do KG,
ou erro meu.

**Why:** KG apontou em 2026-08-21 que minhas mensagens eram prolixas. O argumento decisivo não foi
custo de token — foi **atenção**: *"quando a informação é demais tendemos a não dar atenção e seguir
sem ler tudo"*. Relatório longo torna o achado importante indistinguível do resto. É a mesma falha
que a série de REQs de gate combateu o tempo todo: sinal ruidoso é sinal que ninguém lê, e gate que
ninguém lê não protege nada.

**How to apply:**
- Estrutura padrão de retorno de microlote: **o que mudou · o que decidi · o que preciso de você**.
  Três a cinco linhas.
- **Manter sempre**, mesmo curto: evidência medida (comando + resultado), veredito de barreira,
  decisão tomada e por quê, e erro meu quando houver.
- **Cortar**: repetir o que o executor já relatou, reexplicar racional que já dei antes, recapitular
  o estado quando ele não mudou, e fecho de mensagem elogiando o trabalho.
- Tabela e bloco de código só quando **substituem** prosa, nunca quando somam a ela.
- KG pede profundidade quando quiser ("detalha", "por quê"). Assumir que ele pede se precisar.

Relacionado: [[verificacao-visual-obrigatoria]] — evidência medida continua obrigatória; o que muda
é a extensão do texto ao redor dela, não o rigor.
