---
status: Accepted
date: 2026-10-23
author: "zeus-tf (desenho) / KG (decisão)"
---

# ADR: as regras de escrita dos agentes são aplicadas pelo harness, e não pelo texto do handoff

> Date: 2026-10-23 | Status: Accepted

**REQ:** `docs/req/REQ-2026-10-23-regras-de-escrita-dos-agentes-viram-mecanismo-no-harness-working-context-arquivos-congelados-e-escopo-por-despacho.md`

## Context

As regras de escrita dos subagentes existem hoje apenas como texto: "não escreva no working context", "o único
diretório onde você pode escrever é X", "não toque no arquivo da frente paralela". O texto perde. Casos medidos:

- **2026-10-12:** 5 de 9 subagentes escreveram em `docs/agents-working-context.md`, embora o handoff proibisse.
- **2026-10-21:** o apolo-tf e o hades-tf gravaram no working context do **checkout principal**, e não no worktree
  designado. O resíduo ficou na `main`, onde o guard impede o descarte por `git restore` ou `git stash`.
- **2026-10-23 (REQ-2026-10-23):** o hades-tf anexou ao `docs/agents-working-context.md` já congelado, e o arquiteto teve
  de mover a entrada à mão.

O KG relata o mesmo padrão em outros projetos.

A causa é estrutural. O arquivo de agente gerado pelo trackfw (`internal/integrations/assets/agents/*.md`, seção
`## Working context`) manda o agente registrar início e fim "automaticamente, sem perguntar". Essa instrução de sistema
pesa mais que a proibição no prompt de despacho. E não existe nenhum controle mecânico que segure a escrita, nem na hora
de escrever nem na auditoria.

O Claude Code entrega a identidade do subagente ao hook. Pela documentação oficial
(<https://code.claude.com/docs/en/hooks>), o payload de `PreToolUse` dentro de um subagente traz `agent_id` e `agent_type`,
e o frontmatter do subagente aceita hooks próprios, que rodam só enquanto ele está ativo.

## Decision

Quatro camadas, numa REQ só, entregues nesta ordem (decisão do KG em 2026-10-23):

### D1. A instrução na origem

Nos arquivos de agente gerados, a seção `## Working context` dos papéis que **não** são o arquiteto passa a dizer: "não
escreva no working context; reporte início, fim e o que mudou no seu relatório final; quem registra é o arquiteto". O
arquivo do arquiteto mantém a escrita. Os goldens e os 12 assets mudam juntos.

### D2. A checagem de escopo no `trackfw barrier`

O `trackfw barrier` ganha a checagem `ml_scope`: os arquivos tocados pelos commits da wave só podem ser os declarados em
`**Files affected:**` dos MLs dela, mais um conjunto fixo de artefatos de orquestração (roadmap, REQ, ADR, notas do vault,
`docs/agents-working-context/`). Esta camada vale para qualquer CLI de agente, porque olha o resultado e não o processo.
A Wave 0 define:
- a base do diff da wave;
- a gramática de `Files affected` (caminhos entre crases, globs e diretórios);
- o comportamento com roadmaps antigos que não declaram arquivos, que precisam de uma regra explícita de corte e nunca
  podem passar em silêncio.

### D3. Os caminhos congelados no guard

Um `trackfw guard write-scope`, ligado em `PreToolUse` nos matchers de escrita (`Edit|Write|MultiEdit|NotebookEdit`, e os
equivalentes de cada CLI), nega a escrita em caminhos congelados. A lista padrão inclui o `docs/agents-working-context.md`
legado, e o `trackfw.yaml` pode estendê-la. Esta camada não depende de identidade, então vale também para a sessão
principal.

### D4. O escopo por despacho

O arquiteto grava, antes do despacho, uma permissão de escrita (`.trackfw/leases/<agent_type>.json`) com o worktree e os
caminhos autorizados. O `trackfw guard write-scope` lê o `agent_type` do payload e nega a escrita fora da permissão daquele
agente. Na ausência de permissão para o agente, a escrita segue o comportamento de hoje: liberada, exceto nos caminhos
congelados da D3. Isso evita quebrar o uso sem orquestração. Vale onde o CLI entrega a identidade, o que hoje é o Claude Code.

## Consequences

- A proibição deixa de depender de o agente obedecer: a D3 e a D4 negam, e a D2 reprova antes do commit.
- **Resíduo:** escrita por Bash (`echo >>`, `sed -i`, `python -c`, `cp`) escapa da D3 e da D4. Só a D2 pega, na auditoria. A
  Wave 0 decide se o parser de shell do credential guard cobre os casos mais comuns.
- Nos CLIs sem identidade de subagente, sobram a D1, a D2 e a D3.
- A permissão de escrita é um arquivo que o próprio agente poderia editar. A Wave 0 diz como isso é negado: o arquivo
  entra na lista de congelados para quem não é o arquiteto.

## Alternatives Considered

- **Endurecer só o texto do handoff.** Rejeitada: é o estado atual, e foi medido falhando.
- **Só a barreira de auditoria.** Rejeitada como solução única: ela pega a violação, mas não evita o efeito colateral num
  worktree compartilhado nem no checkout principal.
- **Bloquear toda escrita fora do cwd do agente.** Rejeitada: o cwd do subagente é o do processo pai, e não diz qual
  worktree foi designado.

## Linked REQ
`docs/req/REQ-2026-10-23-regras-de-escrita-dos-agentes-viram-mecanismo-no-harness-working-context-arquivos-congelados-e-escopo-por-despacho.md`
