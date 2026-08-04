---
status: Open
date: 2026-08-04
author: "kg.saran@gmail.com"
adr: ""
roadmap: "docs/roadmaps/backlog/ROADMAP-2026-08-04-discover-init-nao-gera-os-scripts-de-attention-hooks-em-go-e-node-quebra-de-paridade-com-python.md"
---

# REQ: discover --init nao gera os scripts de attention hooks em Go e Node (quebra de paridade com Python)

> Date: 2026-08-04 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation
<!-- Why is this requirement needed? What problem does it solve? -->
Observado ao vivo: rodar `trackfw discover --init` num projeto brownfield gerou `.claude/settings.json`
e `.gemini/settings.json` com hooks `PreToolUse`/`PostToolUse` apontando para
`scripts/trackfw-attention-signal.sh` e `scripts/trackfw-attention-cleanup.sh` — mas nenhum dos dois
arquivos foi criado no repo. Os hooks ficam referenciando scripts inexistentes; quando o Claude Code/
Gemini CLI tentar executá-los, o hook falha silenciosamente ou quebra a chamada da ferramenta.

Causa raiz confirmada por leitura de código, comparando os três runtimes no fluxo `discover --init`:

| Runtime | Chama a geração dos scripts antes de injetar os hooks? |
|---|---|
| Go (`internal/discover/discover.go:InstallGates`) | **Não** — chama só `generators.InjectHooksDetected(rootDir)` (`discover.go:60`), que escreve os `settings.json` referenciando os scripts, mas nunca chama `generateAttentionScripts()` (`internal/generators/scaffold.go:682`, hoje não-exportada) |
| Node.js (`npm/src/commands/discover.js`, bloco `opts.init`) | **Não** — mesma lacuna: chama `injectHooksDetected(cwd)` (linha ~440) sem antes chamar `generateAttentionScripts(cfg, cwd)`, que já existe e é exportada em `npm/src/generators/hooks.js:122` |
| Python (`pypi/trackfw/commands/discover.py`, bloco `opts.init`) | **Sim** — linhas 497-500 chamam `_generate_attention_scripts(cwd)` explicitamente, **antes** de `inject_hooks_detected(cwd)` (linha 503-504) |

Python está correto; Go e Node.js divergem dele. Isso é uma quebra do contrato de paridade
(`docs/cli-parity.md`: "Go is the behavioral reference" — mas aqui é Python quem tem o comportamento
correto, então a referência para esta correção é o comportamento do Python, não o texto genérico da
regra). O caminho greenfield (`trackfw init`, que chama `generateAttentionScripts()` em
`internal/generators/scaffold.go:60` nos três runtimes) não tem esse bug — só o caminho brownfield
(`discover --init`) tem a lacuna, e só em dois dos três runtimes.

## Acceptance Criteria
- [ ] Go: `internal/discover/discover.go:InstallGates` chama a geração dos scripts de attention
      (exportar `generateAttentionScripts` de `internal/generators/scaffold.go` ou expor via
      `internal/generators/hooks.go`) **antes** de `generators.InjectHooksDetected(rootDir)` — mesma
      ordem do Python
- [ ] Node.js: `npm/src/commands/discover.js` chama `generateAttentionScripts(cfg, cwd)`
      (já exportada em `npm/src/generators/hooks.js:122`) **antes** de `injectHooksDetected(cwd)`
- [ ] Python: nenhuma mudança necessária — já correto, serve de referência de comportamento
- [ ] Idempotência preservada: rodar `discover --init` duas vezes não duplica nem corrompe os scripts
      (mesma garantia que `trackfw init`/`trackfw update` já dão)
- [ ] Teste de regressão nos três runtimes: `discover --init` num diretório brownfield simulado deixa
      `scripts/trackfw-attention-signal.sh` e `scripts/trackfw-attention-cleanup.sh` no disco,
      executáveis, com o mesmo conteúdo que `trackfw init` geraria
- [ ] `make quality` verde (Go + Node + Python + contratos de paridade)

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: N/A — bug de paridade entre implementações já existentes, sem decisão arquitetural nova.

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: `docs/roadmaps/backlog/ROADMAP-2026-08-04-discover-init-nao-gera-os-scripts-de-attention-hooks-em-go-e-node-quebra-de-paridade-com-python.md`
