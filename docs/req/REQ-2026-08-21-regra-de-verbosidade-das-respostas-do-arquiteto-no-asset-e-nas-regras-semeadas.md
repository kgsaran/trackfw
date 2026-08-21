---
status: Open
date: 2026-08-21
author: ""
adr: ""
roadmap: ""
---

# REQ: regra de verbosidade das respostas do arquiteto, no asset e nas regras semeadas

> Date: 2026-08-21 | Status: Open

## Motivação

Feedback direto de KG em 2026-08-21, sobre o comportamento do arquiteto ao longo de uma série longa
de microlotes: as respostas eram prolixas e não iam direto ao ponto.

**O argumento decisivo não foi custo de token — foi atenção:**

> *"quando a informação é demais tendemos a não dar atenção e seguir sem ler tudo"*

Relatório longo torna o achado importante **indistinguível do resto**. É exatamente a falha que a
série de REQs de gate combateu o tempo todo, em outra superfície: sinal ruidoso é sinal que ninguém
lê, e o que ninguém lê não protege nada. Um arquiteto que enterra um bloqueio de segurança no
parágrafo sete produziu o mesmo efeito de não tê-lo reportado.

## Decisão de desenho: regra fixa, não botão

KG levantou a hipótese de um controle de verbosidade configurável. **Recomendo regra fixa com
escalada por gatilho**, pelo mesmo motivo que a `REQ-2026-08-18-contrato-pinado` nomeia para o
estado `none`: um botão é ajustado uma vez e esquecido no valor errado — e aí ou o usuário lê demais,
ou o agente esconde um bloqueio. O default precisa ser correto sem intervenção.

## Escopo

A regra, a ser refletida em **dois lugares**:

1. `internal/integrations/assets/agents/architect.md` (+ espelhos npm/pypi) — instrução do próprio
   arquiteto.
2. O `CLAUDE.md` semeado pelo trackfw — gerado por `internal/generators/agentfiles.go`,
   `npm/src/generators/init.js`, `pypi/trackfw/generators/init_gen.py`.

### A regra

- **Padrão curto:** o que mudou · o que decidi · o que preciso de você. Três a cinco linhas.
- **Escala sozinho** em três casos, e só neles: **bloqueio**, **decisão pendente do usuário**, e
  **erro do próprio agente**.
- **Nunca cortar**, mesmo curto: evidência medida (comando + resultado), veredito de barreira,
  decisão tomada e o porquê.
- **Cortar**: repetir o que o executor já relatou, reexplicar racional já dado, recapitular estado
  que não mudou, e fecho elogiando o trabalho.
- Tabela e bloco de código só quando **substituem** prosa, nunca quando somam a ela.
- Profundidade é **sob demanda** do usuário.

## O que **não** é escopo

- Verbosidade dos **executores** ao reportar para o arquiteto. É outro canal, com outro
  destinatário, e o relatório detalhado deles é o que **torna a auditoria possível** — encurtá-lo
  seria perda direta. Se um dia virar problema, é REQ própria.
- Controle configurável de verbosidade. Ver a decisão de desenho acima.
- Mudar o conteúdo técnico do que é reportado. A regra é sobre **extensão**, não sobre rigor.

## 🔴 O risco dominante é o inverso do óbvio

**Encurtar demais esconde bloqueio.** O valor de várias entregas desta semana veio precisamente de
relatório que expôs a própria fragilidade — o executor que declarou *"confirmei no nível topologia,
não alegação-a-asserção, amostre"* permitiu a auditoria que achou o defeito.

Por isso os três gatilhos de escalada não são decorativos: são o que impede a regra de virar
silêncio conveniente.

## Acceptance Criteria

- [ ] AC1 — Regra presente no asset do arquiteto, nos 3 CLIs, byte-idêntica.
- [ ] AC2 — Regra presente no `CLAUDE.md` semeado, nos 3 CLIs, byte-idêntica.
- [ ] AC3 — Os três gatilhos de escalada estão explícitos, e o que **nunca** se corta também.
- [ ] AC4 — Gate comparando as **saídas reais** dos 3 CLIs — o `CLAUDE.md` semeado e o asset já têm
      gate de paridade; estender, não criar paralelo.
- [ ] AC5 — Cenário P4 com baseline e detecção.
- [ ] AC6 — Anotação `trackfw-contract` da seção correspondente atualizada (checker é bloqueante).
- [ ] AC7 — `make quality` verde **e CI verde**.

## Riscos para quem executar

- **Não inventar gate novo** se já existe um cobrindo o asset e o `CLAUDE.md` semeado. Verificar
  antes — houve caso nesta série em que um comparador paralelo quase foi criado sem necessidade.
- **O texto entra nos 3 CLIs**, e é conteúdo literal: divergência de uma vírgula reprova o gate de
  byte-identidade. É esse o ponto dele.
- **Cuidado com o binário do `PATH`** — desatualizado, e `--version` não distingue o build.

## Linked ADR
ADR: <!-- avaliar: a escolha regra-fixa-vs-botao pode merecer registro -->

## Linked Roadmap
Roadmap: <!-- a criar -->
