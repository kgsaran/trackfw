---
status: wip
date: 2026-08-06
req: "docs/req/REQ-2026-08-06-hooks-de-credential-guard-como-escopo-global-cross-project-via-trackfw-update-harness.md"
squad: ""
---

# Roadmap: hooks de credential-guard como escopo global cross-project via trackfw update harness

> Created: 2026-08-06 | Status: wip

## Context
REQ: `docs/req/REQ-2026-08-06-hooks-de-credential-guard-como-escopo-global-cross-project-via-trackfw-update-harness.md`

O credential-guard (PR #141) herdou escopo por-projeto do mecanismo de attention-signal sem isso ser
uma decisão própria — o risco que ele mitiga existe em qualquer projeto, com ou sem `trackfw.yaml`.
ADR `docs/adr/ADR-2026-08-06-hooks-de-credential-guard-em-escopo-global-via-trackfw-update-harness.md`
decide:

1. **Opt-in puro** via `trackfw update harness` — `init`/`update` (escopo de projeto) não mudam de
   comportamento.
2. Script global em `~/.trackfw/scripts/trackfw-credential-guard.sh` (mesmo conteúdo canônico do
   ML-1A original, só muda o destino de escrita).
3. Novos alvos em `HarnessTargetIDs` (`internal/generators/update.go`), um por CLI confirmado:
   Claude Code, Codex, Gemini CLI, Cursor, GitHub Copilot, Kiro (Windsurf continua fora — sem hook
   nativo; Antigravity/Amazon Q/OpenCode fora da wave nativa original, não entram aqui).
4. **Dedup por leitura**: `InjectXHooks` (projeto) detecta wiring global já instalado e pula o
   wiring local do credential-guard especificamente (attention-signal/cleanup continuam normais).
5. Kiro condicionado à v3 (`kiro-cli --v3`) — não instalar silenciosamente.
6. Codex tem uma contradição de documentação (flag `codex_hooks` padrão habilitada vs. opt-in
   explícito) a reconciliar antes de implementar o wiring.

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ] Script global gerado nos 3 stacks, paridade mantida
- [ ] `trackfw update harness` ganha 6 alvos novos (`<tool>-credential-guard`), seguindo o contrato
      já existente (`--targets`/`--install-missing`/`--dry-run`/estados `updated|skipped|missing|failed`)
- [ ] Dedup funcionando: projeto com credential-guard global instalado para um CLI não duplica o
      wiring local desse CLI ao rodar `trackfw init`/`update`, mas mantém attention-signal/cleanup
- [ ] Kiro com verificação de versão v3 antes de instalar (não falha silenciosamente numa v2)
- [ ] Contradição do Codex (flag `codex_hooks`) investigada e resolvida com evidência antes do wiring
- [ ] Gate de paridade estrutural cobrindo os hooks.json/settings.json de escopo harness
- [ ] `trackfw validate`/`make quality` sem regressão

## Wave 1 — Script global + config de modo (1 ML)
> Dependências: Independente

### ML-1A — Gerador do script global + resolução do modo `credential_guard.mode` para escopo harness
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `internal/generators/scaffold.go` (nova função `GenerateGlobalCredentialGuardScript(home string) error`,
  reusa a const `credentialGuardScript` já existente — só muda `scripts/` → `filepath.Join(home,
  ".trackfw", "scripts")`)
- `npm/src/generators/hooks.js` (equivalente `generateGlobalCredentialGuardScript`)
- `pypi/trackfw/generators/init_gen.py` (equivalente `_generate_global_credential_guard_script`)
- Testes irmãos dos já existentes em `credential_guard_test.go`/`credential_guard.test.js`/
  `test_credential_guard.py`
**Ações:**
- Reusar o conteúdo canônico do script (não duplicar a lógica de detecção JWT/AWS-key) — só o
  destino de escrita muda para `~/.trackfw/scripts/trackfw-credential-guard.sh`.
- Decidir e implementar a fonte de `credential_guard.mode` para invocação em escopo global: o script
  hoje lê `trackfw.yaml` na raiz do projeto (`[ -f "trackfw.yaml" ] || exit 0` — ver
  `internal/generators/scaffold.go`, script `credentialGuardScript`). Para uso global, essa guarda de
  "só roda dentro de projeto trackfw" não se aplica. Avaliar: (a) ler `~/.trackfw/config.yaml` (novo
  arquivo, formato mínimo `credential_guard: {mode: warn|block}`) se existir, senão default `warn`;
  ou (b) manter sempre `warn` em escopo global até haver demanda real de configurar `block`
  globalmente — decidir com base no custo de implementação, documentar a escolha no relatório.
- **Não** reescrever a lógica de detecção do script — só o wrapper de geração (destino de arquivo) e,
  se optar por (a), a leitura de config adicional.
**Critérios de aceite:**
- [ ] `GenerateGlobalCredentialGuardScript`/equivalentes escrevem em `~/.trackfw/scripts/` (testado
      com `$HOME` de fixture, nunca o `$HOME` real do ambiente de teste)
- [ ] Paridade de conteúdo entre script de projeto e script global confirmada (mesma lógica de
      detecção, só destino/leitura de config difere)
- [ ] Testes Go/Node/Python verdes
**Comandos de validação:** `go test ./internal/generators/... && npm run test --workspace=npm -- credential_guard && python3 -m pytest pypi/tests/ -k credential_guard`

## Wave 2 — Alvos por CLI em trackfw update harness (6 MLs sequenciais)
> Dependências: Wave 1 completa (script global precisa existir antes de referenciá-lo)

Sequenciais, não paralelos: os alvos novos vivem no mesmo `internal/generators/update.go` (seção
`--- trackfw update harness ---`) e, para os CLIs cujo wiring de hooks já existe
(`internal/generators/agentfiles.go`), tocam esse mesmo arquivo — mesma lição do PR #141 (MLs de
CLI compartilham arquivo, evitar edição concorrente).

### ML-2A — Alvo `claude-credential-guard`
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `internal/generators/update.go` (`harnessCatalogTargetOrder`/`HarnessTargetIDs`, nova função
  `harnessCredentialGuardTarget` ou específica por tool — seguir o padrão de
  `harnessClaudeSkillTarget`/`harnessCatalogTarget` já existentes)
- Testes em `internal/commands/update_harness_test.go`
**Ações:**
- Escrever/mesclar (idempotente, mesmo padrão de `mergeClaudeHookArray`) a entrada de
  `PreToolUse[matcher:"Bash"]`/`PostToolUse[matcher:"Bash"]` em `~/.claude/settings.json`, apontando
  para `~/.trackfw/scripts/trackfw-credential-guard.sh`.
- Estado `missing` se `~/.claude/settings.json` não existir e `--install-missing` não for passado
  (mesmo contrato dos alvos existentes).
**Critérios de aceite:**
- [ ] `trackfw update harness --targets claude-credential-guard --install-missing` cria/mescla a
      entrada corretamente em fixture de `$HOME`
- [ ] Idempotente, `--dry-run` não escreve nada
- [ ] Testes verdes nos 3 stacks
**Comandos de validação:** `go test ./internal/commands/... ./internal/generators/... -run Harness`

### ML-2B — Alvo `codex-credential-guard`
**Status:** ⬜ Pendente
**Arquivos afetados:** mesmos de ML-2A, seção Codex
**Ações:**
- **Investigação obrigatória antes de implementar**: reconciliar a contradição registrada na ADR —
  confirmar via documentação oficial atual do Codex (`developers.openai.com/codex/hooks`,
  `developers.openai.com/codex/config-advanced`) se `[features] codex_hooks` está habilitado por
  padrão ou exige opt-in explícito, e se isso é diferente entre hooks de projeto (`.codex/hooks.json`)
  e hooks globais (`~/.codex/hooks.json`). Se a investigação não resolver com confiança, documentar a
  ambiguidade no output do comando (ex.: mensagem avisando que pode ser necessário habilitar a flag
  manualmente) em vez de assumir.
- Escrever/mesclar `PreToolUse[matcher:"Bash"]`/`PostToolUse[matcher:"Bash"]` em `~/.codex/hooks.json`.
**Critérios de aceite:**
- [ ] Investigação documentada com evidência/fonte no relatório e em `docs/cli-parity.md`
- [ ] `trackfw update harness --targets codex-credential-guard --install-missing` funciona em fixture
- [ ] Testes verdes nos 3 stacks
**Comandos de validação:** `go test ./internal/commands/... ./internal/generators/... -run Harness`

### ML-2C — Alvo `gemini-credential-guard`
**Status:** ⬜ Pendente
**Arquivos afetados:** mesmos de ML-2A, seção Gemini
**Ações:**
- Escrever/mesclar `BeforeTool[matcher:"run_shell_command"]`/`AfterTool[matcher:"run_shell_command"]`
  em `~/.gemini/settings.json`.
**Critérios de aceite:**
- [ ] `trackfw update harness --targets gemini-credential-guard --install-missing` funciona em fixture
- [ ] Testes verdes nos 3 stacks
**Comandos de validação:** `go test ./internal/commands/... ./internal/generators/... -run Harness`

### ML-2D — Alvo `cursor-credential-guard`
**Status:** ⬜ Pendente
**Arquivos afetados:** mesmos de ML-2A, seção Cursor
**Ações:**
- Escrever/mesclar `hooks.beforeShellExecution`/`hooks.afterShellExecution` em `~/.cursor/hooks.json`
  (mesmo schema `{"version":1,"hooks":{...}}` já usado no wiring de projeto, PR #141).
**Critérios de aceite:**
- [ ] `trackfw update harness --targets cursor-credential-guard --install-missing` funciona em fixture
- [ ] Testes verdes nos 3 stacks
**Comandos de validação:** `go test ./internal/commands/... ./internal/generators/... -run Harness`

### ML-2E — Alvo `copilot-credential-guard`
**Status:** ⬜ Pendente
**Arquivos afetados:** mesmos de ML-2A, seção Copilot
**Ações:**
- Confirmar formato exato de `~/.copilot/settings.json` (campo `hooks` inline) via
  `docs.github.com/en/copilot/reference/hooks-configuration` — pode divergir do formato de
  `.github/hooks/*.json` usado no wiring de projeto (arquivo diferente, não necessariamente mesmo
  schema — não assumir, confirmar).
- Escrever/mesclar a entrada apontando para o script global.
**Critérios de aceite:**
- [ ] Formato de `~/.copilot/settings.json` confirmado com fonte, documentado se divergir do formato
      de projeto
- [ ] `trackfw update harness --targets copilot-credential-guard --install-missing` funciona em fixture
- [ ] Testes verdes nos 3 stacks
**Comandos de validação:** `go test ./internal/commands/... ./internal/generators/... -run Harness`

### ML-2F — Alvo `kiro-credential-guard`
**Status:** ⬜ Pendente
**Arquivos afetados:** mesmos de ML-2A, seção Kiro
**Ações:**
- Implementar verificação de versão do Kiro (v3) antes de instalar — investigar como detectar isso
  em runtime (ex.: `kiro --version`, ou arquivo de config que indique a versão) ou, se não for
  detectável de forma confiável, documentar como pré-requisito explícito na mensagem do alvo
  (`TargetResult.Message`) em vez de falhar silenciosamente numa v2.
- Escrever/mesclar hook em `~/.kiro/hooks/` (arquivo dedicado, mesmo padrão do wiring de projeto).
**Critérios de aceite:**
- [ ] Verificação/aviso de versão implementado e testado
- [ ] `trackfw update harness --targets kiro-credential-guard --install-missing` funciona em fixture
- [ ] Testes verdes nos 3 stacks
**Comandos de validação:** `go test ./internal/commands/... ./internal/generators/... -run Harness`

## Wave 3 — Dedup: projeto detecta wiring global (1 ML)
> Dependências: Wave 2 completa (precisa saber o formato exato de cada alvo global para detectá-lo)

### ML-3A — `InjectXHooks` (projeto) pula credential-guard quando já instalado globalmente
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `internal/generators/agentfiles.go` (`InjectClaudeHooks`, `InjectCodexHooks`, `InjectGeminiHooks`,
  `InjectCopilotHooks`, `InjectCursorHooks`, `InjectKiroHooks` — os 6 `InjectXHooks` já existentes)
- Equivalentes Node/Python
**Ações:**
- Para cada um dos 6, antes de adicionar a entrada de credential-guard por-projeto: ler (nunca
  escrever) o arquivo de hooks global correspondente e checar se já existe a entrada apontando para
  `~/.trackfw/scripts/trackfw-credential-guard.sh`. Se sim, pular a adição da entrada de
  credential-guard por-projeto (mas continuar adicionando attention-signal/cleanup normalmente).
- Se o arquivo global não existir ou a leitura falhar por qualquer motivo: tratar como "não
  instalado globalmente" (fail-open para o comportamento atual, nunca fail-closed silenciando o
  credential-guard por-projeto por erro de leitura).
**Critérios de aceite:**
- [ ] Projeto com credential-guard global instalado para um CLI não duplica a entrada de
      credential-guard por-projeto ao rodar `trackfw init`/`update` (fixture com ambos os cenários:
      global instalado / não instalado)
- [ ] attention-signal/cleanup por-projeto continuam sendo adicionados independente do estado global
- [ ] Teste cobrindo o caso de leitura falhando (arquivo global corrompido/inacessível) — confirma
      fail-open
- [ ] Testes verdes nos 3 stacks, `scripts/check-agent-hooks-parity.sh` continua passando
**Comandos de validação:** `go test ./internal/generators/... && npm run test --workspace=npm -- hooks && python3 -m pytest pypi/tests/ -k hooks && GO_BIN=bin/trackfw scripts/check-agent-hooks-parity.sh`

## Wave 4 — Gate de paridade para escopo harness (1 ML)
> Dependências: Wave 2 completa

### ML-4A — Estender gate de paridade estrutural para os alvos harness
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `scripts/check-agent-hooks-parity.sh` (estender) ou novo script dedicado seguindo o mesmo padrão
- `Makefile`, `scripts/check-gates-falsify.sh` (prova negativa, mesmo padrão do Cenário 44)
- `docs/cli-parity.md`
**Ações:**
- Mesmo padrão do `check-agent-hooks-parity.sh` (PR #141), mas com fixture de `$HOME` isolado em vez
  de projeto — gerar os 6 alvos globais via Go/Node/Python reais e comparar estruturalmente.
**Critérios de aceite:**
- [ ] Gate novo/estendido verde para os 6 alvos
- [ ] Prova negativa registrada em `check-gates-falsify.sh`
**Comandos de validação:** `make quality`

## Wave 5 — Documentação e encerramento (1 ML)
> Dependências: Waves 1-4 completas

### ML-5A — Consolidar documentação e fechar REQ
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `docs/cli-parity.md`, `docs/agents-working-context.md`, REQ
**Ações:**
- Documentar os 6 alvos novos, a decisão de dedup, e as duas investigações resolvidas (Codex,
  Kiro v3) numa seção consolidada.
- Atualizar REQ (Acceptance Criteria + Linked Roadmap).
**Critérios de aceite:**
- [ ] `trackfw validate`/`make quality` sem regressão
**Comandos de validação:** `trackfw validate && make quality`
