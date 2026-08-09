# `check-agent-hooks-parity.sh` failed on any dev machine with the global credential-guard already installed — un-isolated `$HOME` — 2026-08-08

## Contexto

`make quality` (ML-4A, gate final da REQ
`REQ-2026-08-08-credential-guard-modo-block-cobertura-read-write-e-resolucao-de-arquivo-referenciado`)
falhou em `check-agent-hooks-parity.sh` com 18 `FAIL` idênticos (`agent-hooks-parity/<cli>/<runtime>/credential-guard-present`:
"scripts/trackfw-credential-guard.sh not referenced anywhere in ...") para os 6 CLIs (claude, codex,
gemini, copilot, cursor, kiro) × 3 runtimes (go, node, py) — falha uniforme nos 3 stacks, o que
descarta regressão de um stack específico e aponta para causa ambiental compartilhada.

## Causa raiz

`InjectClaudeHooks`/`InjectCodexHooks`/.../`InjectKiroHooks` (Go: `internal/generators/agentfiles.go`;
Node: `npm/src/generators/hooks.js`; Python: `pypi/trackfw/generators/hooks.py`) fazem dedup contra o
wiring **global**: se `globalCredentialGuardInstalled<CLI>()` retornar `true` (lê
`$HOME/.trackfw/scripts/trackfw-credential-guard.sh` via `os.UserHomeDir()`/`os.homedir()`/
`os.path.expanduser('~')` — todos respeitam a env var `$HOME`), o wiring de projeto
(Bash/Read/Write|Edit) é **pulado inteiramente**. Essa lógica foi adicionada em ROADMAP-2026-08-06
Wave 3/ML-3A e estendida em ADR-2026-08-06 emenda 7 / ROADMAP-2026-08-08 Wave 1.

`scripts/check-agent-hooks-parity.sh` chama `discover --init` diretamente na fixture
(`run_discover_init`), sem isolar `HOME` — diferente de **todos** os outros gates que invocam um CLI
real (ver `check-update-parity.sh`: `run_update`, `run_init`, `install_agent_*`, todos com
`HOME="$home_dir"` explícito). Em qualquer máquina de desenvolvimento que já rodou `trackfw update
harness` (o próprio onboarding do produto — e este repo/máquina já tinha `~/.claude/settings.json` com
o guard global instalado), o `discover --init` da fixture lia o `$HOME` **real**, via o dedup,
detectava o guard global "instalado" e pulava a entrada de projeto — fazendo o gate falhar
independente do código estar correto.

Mesma causa raiz, instância diferente, de
`vault/notes/node-global-credential-guard-dedup-breaks-inject-tests-on-real-home-2026-08-08.md` (lá
era um teste JS; aqui é um gate shell).

## Fix

`run_discover_init` em `scripts/check-agent-hooks-parity.sh` agora cria `$dir.home` (dir vazio sob
`$WORK`, um por runtime) e passa `HOME="$home_dir"` para as 3 invocações (`go`/`node`/`py`),
igualando o padrão já usado em `check-update-parity.sh`.

## Por que importa para outros gates/testes futuros

- Qualquer gate shell ou teste (em qualquer stack) que invoque `discover --init`, `scaffold`/`init`,
  ou qualquer caminho que passe por `InjectHooksDetected`/`injectHooksDetected`/
  `inject_hooks_detected` e depois afirme presença/ausência de entradas de credential-guard de
  **projeto** precisa isolar `$HOME` — senão o resultado depende de quem/onde roda a suíte, não do
  código. `check-harness-hooks-parity.sh` (escopo global) não tem esse risco por natureza (ele
  instala no `$HOME` isolado de propósito).
- Sintoma de reconhecimento rápido: falha **idêntica nos 3 stacks simultaneamente** em um gate de
  paridade que deveria comparar Go vs Node vs Python — quando os 3 lados falham da mesma forma, a
  causa normalmente não é divergência entre stacks, é ambiente compartilhado (aqui, o `$HOME` real da
  máquina).

## Referências

- `scripts/check-agent-hooks-parity.sh` — `run_discover_init` (fix)
- `scripts/check-update-parity.sh` — padrão de `HOME="$home_dir"` já estabelecido
- `internal/generators/agentfiles.go:1010-1040` (`globalCredentialGuardScriptPath`,
  `readGlobalHookJSON`) — contrato fail-open, leitura via `os.UserHomeDir()`
- `vault/notes/node-global-credential-guard-dedup-breaks-inject-tests-on-real-home-2026-08-08.md` —
  mesma causa raiz, instância Node/teste JS
- `docs/roadmaps/wip/ROADMAP-2026-08-08-credential-guard-modo-block-por-padrao-cobertura-de-read-write-e-resolucao-de-arquivo-referenciado.md`
  (ML-4A)
