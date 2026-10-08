---
status: Open
date: 2026-10-07
author: "zeus-tf"
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-10-07-trackfw-init-nao-instala-os-hooks-de-gemini-e-kiro-na-primeira-execucao-porque-detecta-os-clis-antes-de-criar-seus-arquivos.md"
---

# REQ: trackfw init nao instala os hooks de Gemini e Kiro na primeira execucao porque detecta os CLIs antes de criar seus arquivos

> Date: 2026-10-07 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

Achado do ML-3C da REQ-2026-09-05 na VM (2026-10-06) e reproduzido pelo arquiteto no macOS em 2026-10-07,
com o binário da `main` (782f5767), num diretório vazio com `git init`:

```
trackfw init --ai-tools claude,gemini,kiro --forge github   # 1ª execução, exit 0
  → .gemini/ não existe; .kiro/ não existe
trackfw init --ai-tools claude,gemini,kiro --forge github   # 2ª execução, exit 0
  → .gemini/settings.json criado; .kiro/hooks/ continua ausente
```

Mecanismo relatado pelo ares-tf (a confirmar no código): `scaffold.Scaffold` chama `InjectHooksDetected`
**antes** de `installAITools` criar os arquivos que a detecção procura (ex.: `GEMINI.md`), e o Kiro só recebe
hook se `.kiro/` já existir. O usuário que pediu os três CLIs termina o primeiro `init` sem os guards de
Gemini e Kiro, e sem aviso.

Outra causa que a REQ-2026-09-05 (lá: o hook não executava no Windows; aqui: o hook nem é instalado).

**Escopo negativo:** não muda a linha de hook nem o guard; não muda a detecção de CLIs no `trackfw update`
nem no `discover`, a menos que a medição mostre a mesma causa (aí entra nesta REQ); não trata CLIs fora
dos que o `--ai-tools` aceita.

## Acceptance Criteria
- [ ] AC1 — Causa confirmada no código, com a ordem real das chamadas do `init` e o critério de detecção de cada CLI
- [ ] AC2 — Uma única execução de `trackfw init --ai-tools <lista>` instala o hook de cada CLI pedido (teste por CLI, a partir de diretório vazio)
- [ ] AC3 — Varredura dos outros caminhos que usam a mesma detecção (`update`, `discover`): mesma causa entra aqui, com teste
- [ ] AC4 — Falsificação: voltar a ordem antiga reprova o teste do AC2

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/wip/ROADMAP-2026-10-07-trackfw-init-nao-instala-os-hooks-de-gemini-e-kiro-na-primeira-execucao-porque-detecta-os-clis-antes-de-criar-seus-arquivos.md
