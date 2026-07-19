---
status: Accepted
date: 2026-07-19
author: "Zeus"
---

# ADR: Suporte a ADRs Globais Compartilhados e Diretivas de IA

> Date: 2026-07-19 | Status: Accepted

## Context

Atualmente, o `trackfw` lê decisões arquiteturais (`ADRs`) exclusivamente a partir das pastas mapeadas localmente no repositório. Para grandes organizações, isso gera duplicação de especificações comuns de engenharia (como guias de estilo de linguagem, definições de arquitetura de software e workflows de Git) entre dezenas de microsserviços.

Além disso, assistentes de IA autônomos que operam nos repositórios locais não possuem conhecimento nativo desses padrões externos globais, a menos que o framework os force de maneira ativa a inspecionar esses diretórios corporativos antes de criar ou modificar códigos.

## Decision

1. **Expansão do Til (`~`):** Modificar o resolvedor de caminhos nas três distribuições do CLI (Go, Node.js e Python) para traduzir o atalho `~` para a pasta Home do usuário do sistema operacional em tempo de execução.
2. **Bypass de Portabilidade no CI/CD:** O validador (`trackfw validate`) deve tratar caminhos não existentes listados em `adr_dirs` como `Warning` (em vez de quebrar a validação), a menos que `strict_ci_paths: true` esteja explícito no `trackfw.yaml`.
3. **Regra de Exceção de Órfãos:** Excluir da validação `adr_orphan` os ADRs localizados em caminhos absolutos ou externos ao repositório para evitar quebra de build em projetos locais que não usam todos os padrões corporativos.
4. **Diretiva de Prompt para IA:** Atualizar os geradores do CLI (`scaffold.go` em Go e `init.js` em Node) para injetar uma regra mandatória de sistema em `CLAUDE.md` e `AGENTS.md`, forçando a IA a ler todos os ADRs globais antes de trabalhar em novas REQs e Roadmaps.

## Consequences

* Permite o reaproveitamento de ADRs corporativos comuns.
* Evita falhas de CI/CD em runners que não possuem a pasta Home configurada localmente.
* Garante que os agentes de IA leiam as diretivas globais antes de propor código local.
* Introduz a necessidade de manter a pasta de governança global atualizada no ambiente local do desenvolvedor.

## Alternatives Considered

* **Copiar os ADRs para todos os repositórios:** Rejeitado pelo risco de obsolescência documental rápida (drift).
* **Usar Git Submodules:** Recomendado como melhor prática alternativa, porém o suporte a caminhos de home (`~/`) adiciona flexibilidade para desenvolvedores locais.
