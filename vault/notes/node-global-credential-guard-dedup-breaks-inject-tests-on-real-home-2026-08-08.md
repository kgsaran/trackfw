# Node: testes de injectXHooks que afirmam wiring de projeto quebram silenciosamente se `$HOME` real já tem o credential-guard global instalado — 2026-08-08

## Contexto

ML-3B (ROADMAP-2026-08-08-credential-guard-modo-block-por-padrao-cobertura-de-read-write-e-resolucao-de-arquivo-referenciado.md)
adicionou um teste ponta-a-ponta em `npm/tests/credential_guard.test.js` que chama
`injectClaudeHooks(tmp)` e depois afirma que `.claude/settings.json` ganhou entradas de matcher
`Read`/`Write|Edit` apontando para `scripts/trackfw-credential-guard.sh`. O teste falhava (`PreToolUse
deveria ter entrada com matcher Read`) mesmo com a implementação correta.

## Causa raiz

`injectClaudeHooks` (e o equivalente para Codex/Gemini/Kiro/Copilot/Cursor) faz dedup contra o
wiring **global**: se `globalCredentialGuardInstalledClaude()` retornar `true`, o wiring de projeto
(Bash/Read/Write|Edit) é **pulado inteiramente** — só a entrada `AskUserQuestion` é escrita.
`globalCredentialGuardInstalledClaude()` lê `~/.claude/settings.json` via `os.homedir()`/`$HOME`
**real** da máquina que roda o teste (`npm/src/generators/hooks.js`, `readGlobalHookJSON`). Neste
repo, o hook global já está instalado (é o próprio propósito do produto, e este desenvolvedor rodou
`trackfw update harness` antes) — então qualquer teste que não isole `$HOME` observa o wiring de
projeto sendo silenciosamente pulado e falha, sem nenhuma pista de que a causa é ambiental e não a
implementação.

## Decisão / mitigação já existente

`npm/tests/generators.test.js` já isola isso com `test.beforeEach`/`afterEach` global do arquivo
(linhas ~29-39): salva `$HOME` original, aponta para um `mkdtempSync` vazio antes de cada teste, e
restaura depois. `credential_guard.test.js` usa um runner customizado (não `node:test`), então não
tem hooks globais — a isolação precisa ser feita manualmente, por teste, com `try/finally`:

```js
const origHome = process.env.HOME
process.env.HOME = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-home-iso-'))
try {
  // ... injectXHooks(tmp) + asserções ...
} finally {
  process.env.HOME = origHome
}
```

## Por que importa para outros MLs/stacks

- Qualquer teste **novo** em `npm/tests/credential_guard.test.js` (ou em qualquer arquivo Node que
  não use o `beforeEach` global de `generators.test.js`) que chame `injectXHooks` e afirme presença
  de wiring de projeto **precisa** dessa isolação de `$HOME` — senão passa ou falha dependendo de
  quem/onde roda a suíte, não do código.
- O Python (ML-3C) tem o mesmo padrão de dedup (`globalCredentialGuardInstalled*` equivalente em
  `pypi/trackfw/generators/hooks.py`) e o mesmo risco: se `pypi/tests/test_credential_guard.py`
  também usa um runner sem fixture de isolamento por padrão (ou testa `inject_*_hooks` fora de
  `test_generators_init.py`, que provavelmente já isola via `monkeypatch`/`tmp_path`), aplicar a
  mesma técnica (mock/override de `HOME`/`os.path.expanduser`).
- Go não tem esse problema da mesma forma porque os testes Go de `InjectXHooks`
  (`internal/generators/agentfiles_test.go`) já usam padrão de `t.Setenv("HOME", tmpDir)` — mas
  vale confirmar antes de assumir paridade cega.

## Referências

- `npm/src/generators/hooks.js` — `globalCredentialGuardInstalledClaude`/`Codex`/`Gemini`/`Cursor`/`Copilot`/`Kiro`, `injectClaudeHooks`
- `npm/tests/generators.test.js` linhas ~29-39 (isolamento via `test.beforeEach`/`afterEach`)
- `npm/tests/credential_guard.test.js` (testes "(b)" do ML-3B, primeiros a precisar dessa isolação fora de `generators.test.js`)
- `docs/roadmaps/wip/ROADMAP-2026-08-08-credential-guard-modo-block-por-padrao-cobertura-de-read-write-e-resolucao-de-arquivo-referenciado.md` (ML-3B)
