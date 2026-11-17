---
status: Open
date: 2026-11-17
author: "zeus-tf (pedido do KG)"
adr: ""
roadmap: "docs/roadmaps/backlog/ROADMAP-2026-11-17-guard-de-credencial-no-google-antigravity-e-revisao-do-suporte-ao-gemini-cli.md"
---

# REQ: Guard de credencial no Google Antigravity e revisão do suporte ao Gemini CLI

> Date: 2026-11-17 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation
Em 2026-11-17 o KG informou que o **Gemini CLI não executa mais código** e que o fluxo do Google agora é sempre pelo
**Google Antigravity**. Hoje o trackfw:

- grava o guard de credencial e o guard de git-branch para o Gemini CLI em `.gemini/settings.json`
  (`InjectGeminiHooks`, `internal/generators/agentfiles.go`) e no global, o que deixa de proteger alguma coisa se o agente
  não roda mais comandos;
- **não gera hook nenhum para o Antigravity** (`internal/generators/hooks.go:16`: "CLIs without an Inject*Hooks implementation
  (opencode, antigravity)"). O catálogo (`internal/integrations/assets/catalog.json`) entrega a ele apenas agentes, skills,
  rules e workflows.

O resultado é que quem migrou do Gemini CLI para o Antigravity perdeu o guard de credencial sem nenhum aviso. Ele aparece como
"instalado" para um agente que não executa mais comandos e não existe para o agente que executa.

Descoberto durante a REQ-2026-11-15 (Windows, ML-4A). Fica em REQ própria porque a causa é outra: falta de suporte de hook a um
agente novo, e não comportamento de plataforma.

## Acceptance Criteria
- [ ] Medido, em fonte primária (documentação oficial e binário instalado): o Gemini CLI atual ainda executa shell/ferramentas?
  Se não, desde qual versão. A medição tem um contra-braço.
- [ ] Medido: o Antigravity tem mecanismo de hook ou interceptação antes de executar comando (nome do evento, formato, arquivo,
  escopo projeto/global, shell usado no Windows e no macOS). Se não tiver, isso fica escrito com a fonte, e a REQ define o que o
  trackfw entrega no lugar (por exemplo, um aviso no `doctor`/`validate` de que o Antigravity roda sem guard).
- [ ] Se houver hook: `trackfw init`/`update` gravam o guard de credencial para o Antigravity, e o `doctor` audita no mesmo
  caminho. Teste e medição real no Antigravity com um segredo fake bloqueado e um comando inócuo liberado.
- [ ] Decisão registrada em ADR sobre o Gemini CLI: manter, marcar como legado ou remover os hooks. Se a decisão for remover,
  ela inclui a migração das instalações existentes.
- [ ] `docs/cli-parity.md` e o catálogo refletem o estado medido.

## Negative scope
- Não altera o comportamento de Windows de outros CLIs (isso é da REQ-2026-11-15).
- Não cria guard para OpenCode, mesmo sendo o outro CLI sem hook. Se a investigação mostrar a mesma causa, ele entra como ML
  desta REQ, pela Regra Dura de Causa Raiz, mas não está no escopo inicial.
- Não remove o suporte ao Gemini CLI sem a ADR do critério acima.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/backlog/ROADMAP-2026-11-17-guard-de-credencial-no-google-antigravity-e-revisao-do-suporte-ao-gemini-cli.md
