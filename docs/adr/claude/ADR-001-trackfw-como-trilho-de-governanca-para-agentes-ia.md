# ADR-001 — trackfw como trilho de governança para desenvolvimento orquestrado por agentes de IA

**Status:** Accepted  
**Data:** 2026-06-13  
**Autor:** Zeus (Principal Software Architect)  
**REQ:** REQ-2026-06-13-trackfw-ai-agent-governance-rail.md

---

## Contexto

A análise competitiva de mercado (2026-06-13) identificou um white space estratégico:

- **OpenSpec** e **GSD** provam que há demanda crescente por estruturas formais que *agentes de IA seguem* durante o desenvolvimento, mas ambos param na spec ou no contexto do agente — sem rastreabilidade formal de decisão→requisito→entrega.
- O **trackfw** já nasce nesse contexto operacional: o próprio projeto é desenvolvido por múltiplos agentes especializados (Zeus, Apolo, Afrodite, Artemis…) que seguem a cadeia `ADR → REQ → ROADMAP → kanban` como trilho de orquestração.
- Nenhum concorrente liga, num único artefato verificável em CI, **decisão arquitetural → requisito de negócio → estado de entrega** de forma que um agente de IA possa consumir, produzir e validar autonomamente.

Até a v2.0.0, o trackfw foi posicionado implicitamente como "CLI de governança para times humanos". A realidade do uso, porém, mostra que o segmento de **desenvolvimento orquestrado por agentes de IA** é o diferenciador de maior upside e o único onde não existe competição direta.

## Decisão

O trackfw adota formalmente o posicionamento de **trilho de governança para desenvolvimento orquestrado por agentes de IA**, sem abrir mão do público de times humanos (que permanece como base).

Isso implica:

### 1. A cadeia ADR→REQ→ROADMAP é o contrato entre agentes

Cada artefato da cadeia serve como handoff formal entre agentes:
- **ADR** = decisão arquitetural aprovada (input para qualquer agente que vai implementar algo)
- **REQ** = requisito de negócio rastreável à decisão (briefing estruturado para agente implementador)
- **ROADMAP** = plano de microlotes com critérios de aceite mensuráveis (instruções executáveis por agente)
- **Estado kanban** = fonte de verdade do progresso (backlog → wip → done)

### 2. `trackfw validate` como gate de conformidade de agente

Antes de um agente implementador iniciar trabalho, `trackfw validate` deve passar: garante que o ROADMAP tem REQ vinculada, a REQ tem ADR, e os ADRs não estão em Draft. Isso previne agentes de implementar sem decisão aprovada.

### 3. Geração de artefatos legíveis por máquina

Os artefatos do trackfw (ADR, REQ, ROADMAP) devem ser estruturados o suficiente para que um agente de IA:
- **Consuma** um ROADMAP e execute os MLs autonomamente
- **Produza** um ADR ou REQ a partir de um prompt de contexto
- **Valide** a cadeia via `trackfw validate` sem intervenção humana

### 4. `trackfw discover` como onboarding de agente em repositório desconhecido

Quando um agente de IA é invocado num repositório sem contexto, `trackfw discover` é o primeiro comando a executar: entrega GovernanceScore, paths calibrados e `.trackfw-log` retroativo — reduzindo o tempo de orientação do agente de minutos para segundos.

### 5. Paridade Go CLI + npm CLI como requisito inviolável

Agentes de IA operam em contextos heterogêneos (Go toolchain, Node.js, containers sem Go). A paridade total garante que o trilho funcione independentemente do runtime do agente.

## Alternativas consideradas

| Alternativa | Razão para rejeitar |
|---|---|
| Manter posicionamento implícito (não declarar) | Perde a janela de diferenciação; OpenSpec/GSD vão preencher o espaço se o trackfw não o reivindicar |
| Criar um produto separado "trackfw-ai" | Fragmenta o ecossistema e dobra o custo de manutenção; a cadeia já existente é o diferenciador |
| Focar apenas em times humanos | Mercado saturado por Jira/Linear/GitHub Projects; sem vantagem competitiva clara |
| Adicionar camada MCP/API para agentes consumirem | Complementar — pode vir depois, mas não é o passo inicial; a interface de arquivo Markdown já é consumível por LLMs |

## Consequências

### Positivas
- Diferenciação clara e defensável num segmento sem competição direta
- Valida o uso interno (o próprio trackfw é desenvolvido com trackfw por agentes)
- Direciona roadmap: features que reduzem atrito de agentes têm prioridade
- Abre casos de uso em times com coding agents como Claude Code, Gemini CLI, Cursor

### Negativas / Riscos
- Segmento de "desenvolvimento por agentes" ainda é nascente — adoção pode ser lenta
- Requer que os artefatos (ADR/REQ/ROADMAP) sejam suficientemente estruturados para consumo por LLM — pressão adicional de qualidade de template
- Risco de over-engineering: adicionar complexidade "para agentes" que times humanos não querem

### Neutras
- O posicionamento não exclui times humanos — é uma ampliação, não uma pivotagem
- Features como `--brownfield`, non-TTY flags e discovery já servem tanto agentes quanto CI headless

## Roadmap decorrente

Ver: `docs/roadmaps/claude/backlog/trackfw-ai-agent-rail-2026-06-13.md`

Features priorizadas por este ADR:
1. Templates de ADR/REQ/ROADMAP com frontmatter estruturado (YAML parseable por agentes)
2. `trackfw context` — comando que emite um dump de contexto de governança consumível por LLM (ADRs aceitos + REQs abertas + WIP atual)
3. `trackfw roadmap new --from-req` com geração assistida de MLs a partir do conteúdo da REQ
4. Modo MCP server (`trackfw serve --mcp`) — expõe cadeia como recursos MCP para coding agents
5. Schema JSON/YAML para validação programática de ADR/REQ/ROADMAP por agentes

---

*"A cadeia ADR→REQ→ROADMAP não é apenas documentação — é o protocolo de handoff entre agentes."*
