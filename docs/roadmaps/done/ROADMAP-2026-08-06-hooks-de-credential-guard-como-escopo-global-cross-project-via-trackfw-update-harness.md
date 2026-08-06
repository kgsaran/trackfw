---
status: done
date: 2026-08-06
req: "docs/req/REQ-2026-08-06-hooks-de-credential-guard-como-escopo-global-cross-project-via-trackfw-update-harness.md"
squad: ""
---

# Roadmap: hooks de credential-guard como escopo global cross-project via trackfw update harness

> Created: 2026-08-06 | Status: done

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
- [x] Dedup funcionando: projeto com credential-guard global instalado para um CLI não duplica o
      wiring local desse CLI ao rodar `trackfw init`/`update`, mas mantém attention-signal/cleanup
- [ ] Kiro com verificação de versão v3 antes de instalar (não falha silenciosamente numa v2)
- [ ] Contradição do Codex (flag `codex_hooks`) investigada e resolvida com evidência antes do wiring
- [ ] Gate de paridade estrutural cobrindo os hooks.json/settings.json de escopo harness
- [ ] `trackfw validate`/`make quality` sem regressão

## Wave 1 — Script global + config de modo (1 ML)
> Dependências: Independente

### ML-1A — Gerador do script global + resolução do modo `credential_guard.mode` para escopo harness
**Status:** ✅ Concluído

**Nota de auditoria:** script decomposto em blocos componíveis (header + guarda-de-projeto +
núcleo-de-detecção compartilhado + tail-de-projeto/tail-global) nos 3 stacks — núcleo de detecção
nunca duplicado. Confirmado pelo orquestrador, de forma independente do relato do agente, que o
script de PROJETO continua **byte-idêntico** ao gerado antes desta refatoração (`git stash` +
comparação direta dos dois binários). Decisões: modo sempre `warn` em escopo global (sem
`~/.trackfw/config.yaml` — complexidade não demandada, revisável se houver demanda real);
`ROADMAP_DIR` fixo em `docs/roadmaps` relativo ao cwd, só grava o attention se o diretório já
existir (não cria `docs/roadmaps` num projeto aleatório). Testes confirmados usando `$HOME` de
fixture (`t.TempDir()`/equivalentes) — nunca o `$HOME` real do ambiente.
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
**Status:** ✅ Concluído

**Achado corrigido durante a execução:** o ML original só listava arquivos Go — erro do orquestrador
na autoria do roadmap, violando a regra dura de paridade 3 CLIs do `CLAUDE.md`. O agente Go
implementou corretamente só o que foi pedido e sinalizou a violação em vez de expandir escopo por
conta própria (ao rodar `make quality`/`check-update-parity.sh`). Corrigido com um follow-up dedicado
cobrindo `npm/src/commands/update-harness.js` e `pypi/trackfw/commands/update_harness.py`, espelhando
fielmente a implementação Go já aprovada. `check-update-parity.sh` confirmado verde com os 22 ids
idênticos nos 3 stacks. `docs/cli-parity.md` ("21 ids") corrigido para 22. Todos os MLs seguintes
(2B-2F) já foram corrigidos no roadmap para exigir os 3 stacks explicitamente desde o início — ver
[[feedback_roadmap_deve_listar_3_stacks]] na memória do orquestrador.

**Arquivos afetados (Go, já implementado + Node/Python, follow-up):**
- `internal/generators/update.go` (`harnessCatalogTargetOrder`/`HarnessTargetIDs`, nova função
  `harnessCredentialGuardTarget` ou específica por tool — seguir o padrão de
  `harnessClaudeSkillTarget`/`harnessCatalogTarget` já existentes)
- `npm/src/commands/update-harness.js` (`HARNESS_TARGET_IDS`, lógica de instalação equivalente)
- `pypi/trackfw/commands/update_harness.py` (equivalente)
- Testes em `internal/commands/update_harness_test.go` + testes irmãos Node/Python
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
**Status:** ✅ Concluído

**Investigação da contradição (resolvida com alta confiança):** re-fetch direto em 2026-08-06 de
`https://developers.openai.com/codex/hooks` confirma: "Hooks are enabled by default. To turn them off
in config.toml, set: `[features] hooks = false`. Use `hooks` as the canonical feature key.
`codex_hooks` still works as a deprecated alias." `https://developers.openai.com/codex/config-
advanced` (mesma data) não tem requisito conflitante. Ou seja: `codex_hooks`/`hooks` só serve para
DESLIGAR hooks — nunca é necessário como opt-in, nem para wiring de projeto nem global. Nenhuma
`Message` de aviso extra foi adicionada ao `TargetResult` (a investigação resolveu com confiança
total, então a hedge condicional do próprio ML não se aplica). Evidência completa registrada em
`docs/cli-parity.md`, seção "Declared harness targets — pinned list".
**Posição em `HarnessTargetIDs`:** `codex-credential-guard` inserido imediatamente ANTES de
`codex-agents`/`codex-skills` — mesma posição relativa de `claude-credential-guard` (que precede
`claude-agents`/`claude-skills`). Lista completa agora com 23 ids, confirmada idêntica nos 3 stacks
por `check-update-parity.sh` (cenário `target-list/three-runtimes-identical`).
**Arquivos afetados:** mesmos 3 stacks de ML-2A (`internal/generators/update.go` +
`npm/src/commands/update-harness.js` + `pypi/trackfw/commands/update_harness.py`, e os testes
irmãos), seção Codex — **regra dura de paridade 3 CLIs é obrigatória neste ML** (o ML-2A ficou
Go-only por erro do orquestrador na autoria do roadmap; não repetir)
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
**Status:** ✅ Concluído

**Posição em `HarnessTargetIDs`/`HARNESS_TARGET_IDS`/`declared_target_ids()`:**
`gemini-credential-guard` inserido imediatamente ANTES de `gemini-agents`/`gemini-skills` — mesma
posição relativa de `claude-credential-guard`/`codex-credential-guard`. Lista completa agora com 24
ids, confirmada idêntica nos 3 stacks por `check-update-parity.sh` (cenário
`target-list/three-runtimes-identical`). `docs/cli-parity.md` ("23 ids") corrigido para 24.
**Helper de merge:** `mergeClaudeHookArray` (Go) / `mergeClaudeHookArray` (Node,
`generators/hooks.js`) / `_merge_claude_hook_array` (Python, `generators/hooks.py`) funcionou sem
adaptação — mesmo shape de array `[{matcher, hooks:[{type,command}]}]` usado por
`InjectGeminiHooks`/equivalentes de projeto, só mudam os nomes das chaves de topo (`BeforeTool`/
`AfterTool` em vez de `PreToolUse`/`PostToolUse`) e o matcher (`run_shell_command` em vez de `Bash`).
Novo wrapper `mergeCredentialGuardGeminiHooks` (Go) e lógica equivalente inline em Node/Python criados
só para essas chaves de topo — reaproveitando o helper de array por baixo.
**Arquivos afetados:**
- `internal/generators/update.go` (`buildHarnessTargetIDs`, `UpdateHarness` dispatcher,
  `mergeCredentialGuardGeminiHooks`, `harnessCredentialGuardTargetGemini`)
- `internal/generators/update_test.go` (5 testes espelhando os de Codex: missing, install absolute
  path, idempotência, dry-run, preservação de conteúdo pré-existente)
- `internal/commands/update_harness_test.go` (`TestUpdateHarnessCmd_CredentialGuardGeminiInstallsViaCLI`)
- `npm/src/commands/update-harness.js` (`HARNESS_TARGET_IDS`, `credentialGuardTargetGemini`,
  dispatcher em `buildHarnessTargets`)
- `npm/tests/update-harness.test.js` (5 testes espelhando os de Codex)
- `pypi/trackfw/commands/update_harness.py` (`declared_target_ids`, `_credential_guard_gemini_result`,
  dispatcher em `_run`)
- `pypi/tests/test_update_harness.py` (5 testes espelhando os de Codex + ajuste de
  `test_harness_declared_target_list_and_order` para 24 ids)
- `docs/cli-parity.md` (seção "Declared harness targets — pinned list" corrigida para 24 ids)
**Ações:**
- Escrever/mesclar `BeforeTool[matcher:"run_shell_command"]`/`AfterTool[matcher:"run_shell_command"]`
  em `~/.gemini/settings.json`.
**Critérios de aceite:**
- [x] `trackfw update harness --targets gemini-credential-guard --install-missing` funciona em fixture
- [x] Testes verdes nos 3 stacks
**Comandos de validação:** `go test ./internal/commands/... ./internal/generators/... -run Harness`

### ML-2D — Alvo `cursor-credential-guard`
**Status:** ✅ Concluído

**Helper de merge:** `mergeSimpleCommandArray` (Go, `agentfiles.go`) — não `mergeClaudeHookArray`.
Confirmado lendo `InjectCursorHooks` (projeto) inteira antes de implementar: Cursor's hooks.json
usa `{"version":1,"hooks":{"<event>":[{"command":"..."}]}}` — cada entrada de evento é um objeto
plano `{"command": "..."}`, SEM `matcher`, sem `{type, hooks:[...]}` aninhado como
Claude/Codex/Gemini. Node.js ganhou um `mergeSimpleCommandArray` equivalente exportado de
`generators/hooks.js` (não existia; extraído da lógica já inline em `injectCursorHooks`). Python
ganhou `_merge_simple_command_array` equivalente em `generators/hooks.py`.
**Posição em `HarnessTargetIDs`/`HARNESS_TARGET_IDS`/`declared_target_ids()`:**
`cursor-credential-guard` inserido imediatamente ANTES de `cursor-agents`/`cursor-skills` — mesma
posição relativa dos MLs anteriores. Lista completa agora com 25 ids, confirmada idêntica nos 3
stacks por `check-update-parity.sh` (cenário `target-list/three-runtimes-identical`).
`docs/cli-parity.md` ("24 ids") corrigido para 25.
**Arquivos afetados:**
- `internal/generators/update.go` (`buildHarnessTargetIDs`, `UpdateHarness` dispatcher,
  `mergeCredentialGuardCursorHooks`, `harnessCredentialGuardTargetCursor`)
- `internal/generators/update_test.go` (5 testes espelhando os de Gemini: missing, install absolute
  path, idempotência, dry-run, preservação de conteúdo pré-existente — asserções lêem
  `hooks[event]` como array plano de `{command}`, sem `matcher`)
- `internal/commands/update_harness_test.go` (`TestUpdateHarnessCmd_CredentialGuardCursorInstallsViaCLI`)
- `npm/src/generators/hooks.js` (`mergeSimpleCommandArray`, exportado)
- `npm/src/commands/update-harness.js` (`HARNESS_TARGET_IDS`, `credentialGuardTargetCursor`,
  dispatcher em `buildHarnessTargets`)
- `npm/tests/update-harness.test.js` (5 testes espelhando os de Gemini)
- `pypi/trackfw/generators/hooks.py` (`_merge_simple_command_array`)
- `pypi/trackfw/commands/update_harness.py` (`declared_target_ids`,
  `_credential_guard_cursor_result`, dispatcher em `_run`)
- `pypi/tests/test_update_harness.py` (5 testes espelhando os de Gemini + ajuste de
  `test_harness_declared_target_list_and_order` para 25 ids)
- `docs/cli-parity.md` (seção "Declared harness targets — pinned list" corrigida para 25 ids)
**Ações:**
- Escrever/mesclar `hooks.beforeShellExecution`/`hooks.afterShellExecution` em `~/.cursor/hooks.json`
  (mesmo schema `{"version":1,"hooks":{...}}` já usado no wiring de projeto, PR #141).
**Critérios de aceite:**
- [x] `trackfw update harness --targets cursor-credential-guard --install-missing` funciona em fixture
- [x] Testes verdes nos 3 stacks
**Comandos de validação:** `go test ./internal/commands/... ./internal/generators/... -run Harness`

### ML-2E — Alvo `copilot-credential-guard`
**Status:** ✅ Concluído

**Investigação do formato (confirmada com fonte, 2026-08-06):** re-fetch direto de
`https://docs.github.com/en/copilot/reference/hooks-reference` (a URL `hooks-configuration` do ML
301-redireciona para esta — mesma página já usada na investigação de escopo de projeto), seção
"Hooks locations". O escopo de usuário/global tem DOIS mecanismos distintos: (1) um diretório
dedicado de arquivos de hook (`~/.copilot/hooks/*.json`, análogo user-scope de
`.github/hooks/*.json`), e (2) "Inline hooks block in user-level config — the hooks field at the top
level of `~/.copilot/settings.json`". Seguido o mecanismo (2), conforme instruído pelo roadmap.
Confirmado que `~/.copilot/settings.json` NÃO é um arquivo dedicado a hooks — é o arquivo de config
geral do usuário do Copilot CLI (guarda outras chaves, ex. `model`) — logo a wiring faz **merge**
em `root["hooks"]["preToolUse"/"postToolUse"]`, preservando qualquer outra chave de topo, igual ao
tratamento que `claude-credential-guard`/`codex-credential-guard`/`gemini-credential-guard` já dão
aos seus próprios arquivos de settings gerais (diferente de Cursor, cujo `~/.cursor/hooks.json` é
ele mesmo um arquivo dedicado). Shape da entrada confirmado idêntico ao de escopo de projeto
(`InjectCopilotHooks`): "Hook configuration files use JSON format with version 1" é declarado sem
ressalva para o campo `hooks` inline, e nenhum exemplo do doc mostra shape diferente para
`settings.json` — reutilizado `{"type":"command","matcher":"bash","bash":"<caminho absoluto>",
"cwd":".","timeoutSec":10}` em `hooks.preToolUse`/`hooks.postToolUse`. Decisão deliberada: **nenhuma
chave `"version"` de topo é adicionada** — todo exemplo do doc com `"version":1` é de um arquivo
DEDICADO a hooks (`.github/hooks/*.json`, arquivos de policy); nenhum exemplo mostra `"version"` no
próprio `settings.json`, então adicioná-la seria uma suposição não confirmada sobre um arquivo que
este código não possui integralmente (mesma lógica já aplicada a Claude/Codex/Gemini). Evidência
completa registrada em `docs/cli-parity.md`, seção "GitHub Copilot global-scope wiring (ML-2E)".
**Posição em `HarnessTargetIDs`/`HARNESS_TARGET_IDS`/`declared_target_ids()`:**
`copilot-credential-guard` inserido imediatamente ANTES de `copilot-agents`/`copilot-skills` — mesma
posição relativa dos MLs anteriores. Lista completa agora com 26 ids, confirmada idêntica nos 3
stacks por `check-update-parity.sh` (cenário `target-list/three-runtimes-identical`).
`docs/cli-parity.md` ("25 ids") corrigido para 26.
**Helper de merge:** shape distinto de Cursor (`{"command":...}` plano) e de Claude/Codex/Gemini
(`{"matcher","hooks":[...]}` aninhado): Copilot usa `{"type","matcher":"bash","bash","cwd",
"timeoutSec"}` plano, casado pelo campo `"bash"`. Go reaproveita `mergeSimpleCommandArray` com
`getCmd`/`makeEntry` customizados; Node ganhou `mergeCopilotHookArray` dedicado (exportado de
`generators/hooks.js`); Python ganhou `_merge_copilot_hook_array` dedicado
(`generators/hooks.py`).
**Arquivos afetados:**
- `internal/generators/update.go` (`buildHarnessTargetIDs`, `UpdateHarness` dispatcher,
  `mergeCredentialGuardCopilotHooks`, `harnessCredentialGuardTargetCopilot`)
- `internal/generators/update_test.go` (5 testes espelhando os de Cursor: missing, install absolute
  path, idempotência, dry-run, preservação de conteúdo pré-existente — sem chave `"version"`)
- `internal/commands/update_harness_test.go` (`TestUpdateHarnessCmd_CredentialGuardCopilotInstallsViaCLI`)
- `npm/src/generators/hooks.js` (`mergeCopilotHookArray`, exportado)
- `npm/src/commands/update-harness.js` (`HARNESS_TARGET_IDS`, `credentialGuardTargetCopilot`,
  dispatcher em `buildHarnessTargets`)
- `npm/tests/update-harness.test.js` (5 testes espelhando os de Cursor)
- `pypi/trackfw/generators/hooks.py` (`_merge_copilot_hook_array`)
- `pypi/trackfw/commands/update_harness.py` (`declared_target_ids`,
  `_credential_guard_copilot_result`, dispatcher em `_run`)
- `pypi/tests/test_update_harness.py` (5 testes espelhando os de Cursor + ajuste de
  `test_harness_declared_target_list_and_order` para 26 ids)
- `docs/cli-parity.md` (seção "Declared harness targets — pinned list" corrigida para 26 ids +
  nova seção "GitHub Copilot global-scope wiring (ML-2E)" com a investigação completa)
**Ações:**
- Escrever/mesclar `hooks.preToolUse`/`hooks.postToolUse[matcher:"bash"]` em
  `~/.copilot/settings.json` (mesmo shape de entrada do wiring de projeto, ML-2D de
  agentfiles.go), sem adicionar chave `"version"` de topo.
**Critérios de aceite:**
- [x] Formato de `~/.copilot/settings.json` confirmado com fonte, documentado (converge com o
      shape de projeto; diverge em NÃO ser um arquivo dedicado — precisa merge/preservação)
- [x] `trackfw update harness --targets copilot-credential-guard --install-missing` funciona em fixture
- [x] Testes verdes nos 3 stacks
**Comandos de validação:** `go test ./internal/commands/... ./internal/generators/... -run Harness`

### ML-2F — Alvo `kiro-credential-guard`
**Status:** ✅ Concluído

**Investigação de versão v3 (confirmada 2026-08-06, `curl -L` contra `kiro.dev/changelog/cli/2-13/` e
`kiro.dev/docs/cli/`):** `--v3` é uma flag de MODO DE LANÇAMENTO do mesmo binário instalado
("Available in V3 (`kiro-cli --v3`)"), não um valor que algum comando `--version` reporta — nenhuma
das duas páginas documenta flag `--version`/formato de saída para o CLI. Não há, portanto, um fato de
versão instalada persistente para sondar de um processo externo (trackfw nunca invoca o Kiro
diretamente); se uma sessão Kiro respeita o arquivo global depende de como o usuário lança a PRÓXIMA
sessão (`kiro-cli --v3`), não de nada em disco agora. Decisão: **sem sonda de subprocesso** — e sem
usar `TargetResult.Message` para o aviso (confirmado que o contrato é failure-only: struct
`TargetResult.Message` documentado "only set when State == TargetFailed",
`TestUpdateHarnessCmd_JSONKeyOrderMatchesCliParityContract` reprova qualquer `message` fora de
`failed`). Pré-requisito documentado em `docs/cli-parity.md` (nova seção "Kiro global-scope wiring
(ML-2F)") + doc comments nos 3 stacks.
**Formato do arquivo global confirmado:** `~/.kiro/hooks/trackfw-credential-guard.json` — arquivo
DEDICADO (um arquivo por hook no diretório `~/.kiro/hooks/`, não um settings.json geral compartilhado
como Claude/Codex/Gemini/Copilot), confirmado pelo mesmo changelog: "Hooks placed in ~/.kiro/hooks/
now fire in every workspace automatically". Escrito por sobrescrita total (nunca merge) — mesmo
padrão de `claude-skill`, não o padrão merge-and-preserve dos alvos de settings.json. Schema idêntico
ao `InjectKiroHooks` de projeto (`{"version":"v1","hooks":[...]}`), mas `command` com caminho
ABSOLUTO e nomes de hook distintos (`trackfw-credential-guard-global-pre`/`-global-post`) dos nomes
de projeto, para não apostar em comportamento de dedup-por-nome do Kiro não documentado entre
arquivos/escopos diferentes.
**Posição em `HarnessTargetIDs`:** `kiro-credential-guard` inserido imediatamente ANTES de
`kiro-agents`/`kiro-skills` — mesma posição relativa dos MLs anteriores, e último
`<tool>-credential-guard` da wave (26→27 ids, confirmado idêntico nos 3 stacks por
`check-update-parity.sh`).
**Arquivos afetados:**
- `internal/generators/update.go` (`buildHarnessTargetIDs`, `UpdateHarness` dispatcher,
  `harnessCredentialGuardTargetKiro`)
- `internal/generators/update_test.go` (5 testes: missing, install absolute path, idempotência,
  dry-run, rewrite de conteúdo obsoleto — arquivo dedicado, nunca merge)
- `internal/commands/update_harness_test.go` (`TestUpdateHarnessCmd_CredentialGuardKiroInstallsViaCLI`)
- `npm/src/commands/update-harness.js` (`HARNESS_TARGET_IDS`, `credentialGuardTargetKiro`,
  dispatcher em `buildHarnessTargets`)
- `npm/tests/update-harness.test.js` (5 testes espelhando os do Go)
- `pypi/trackfw/commands/update_harness.py` (`declared_target_ids`, `_credential_guard_kiro_result`,
  dispatcher em `_run`)
- `pypi/tests/test_update_harness.py` (5 testes + ajuste de
  `test_harness_declared_target_list_and_order` para 27 ids)
- `docs/cli-parity.md` (lista pinada corrigida para 27 ids + nova seção "Kiro global-scope wiring
  (ML-2F)")
**Critérios de aceite:**
- [x] Verificação/aviso de versão implementado e testado (documentado, sem sonda de subprocesso)
- [x] `trackfw update harness --targets kiro-credential-guard --install-missing` funciona em fixture
- [x] Testes verdes nos 3 stacks
**Comandos de validação:** `go test ./internal/commands/... ./internal/generators/... -run Harness`

## Wave 3 — Dedup: projeto detecta wiring global (1 ML)
> Dependências: Wave 2 completa (precisa saber o formato exato de cada alvo global para detectá-lo)

### ML-3A — `InjectXHooks` (projeto) pula credential-guard quando já instalado globalmente
**Status:** ✅ Concluído

**Implementação (3 stacks):** para cada um dos 6 `InjectXHooks`/`injectXHooks`/`inject_x_hooks`,
antes de adicionar a entrada de credential-guard por-projeto, uma checagem read-only lê (nunca
escreve) o arquivo de hooks global correspondente e verifica se já existe a entrada apontando para
`~/.trackfw/scripts/trackfw-credential-guard.sh` (path absoluto, `$HOME` resolvido via
`os.UserHomeDir()`/`os.homedir()`/`os.path.expanduser('~')`, nunca uma string literal com `~`):
- **Claude/Codex** (`.claude/settings.json`/`.codex/hooks.json`): `hooks.PreToolUse[matcher:"Bash"]`.
- **Gemini** (`.gemini/settings.json`): `hooks.BeforeTool[matcher:"run_shell_command"]`.
- **Cursor** (`.cursor/hooks.json`): `hooks.beforeShellExecution[].command` (array plano, sem
  matcher).
- **Copilot** (`.copilot/settings.json`): `hooks.preToolUse[].bash` (array plano, matched pelo campo
  `"bash"`).
- **Kiro** (`.kiro/hooks/trackfw-credential-guard.json`): arquivo 100% dedicado (nunca merge) — basta
  checar existência + conteúdo não-vazio (`os.Stat`/`fs.statSync`/`os.path.getsize`).

Se a entrada global já existe: a entrada de credential-guard por-projeto é pulada (Bash/BeforeTool/
beforeShellExecution+afterShellExecution/preToolUse+postToolUse conforme o CLI) — attention-signal/
attention-cleanup continuam sendo adicionados normalmente, sem alteração, em todos os 6 CLIs.

**Fail-open confirmado**: qualquer falha ao resolver `$HOME`, ler o arquivo global (inexistente,
sem permissão) ou parsear o JSON (corrompido) é tratada como "não instalado globalmente" — a entrada
de credential-guard por-projeto continua sendo adicionada exatamente como antes deste ML. Nenhum
caminho de leitura do estado global pode silenciar o credential-guard por-projeto por erro.

**Arquivos alterados:**
- `internal/generators/agentfiles.go` (bloco novo "Global credential-guard dedup" — helpers
  `globalCredentialGuardScriptPath`, `readGlobalHookJSON`, `hookArrayHasCommand`,
  `simpleArrayHasValue`, e uma `globalCredentialGuardInstalled<Tool>` por CLI — e os 6 `InjectXHooks`
  passam a checar antes de mesclar/anexar a entrada de credential-guard).
- `internal/generators/agentfiles_test.go` (10 testes existentes que chamam `InjectXHooks` ganharam
  `t.Setenv("HOME", t.TempDir())` para isolar a nova checagem do `$HOME` real da máquina —
  hermeticidade, não regressão funcional).
- `internal/generators/hooks_test.go`, `internal/generators/copilot_hooks_parity_test.go`,
  `internal/generators/credential_guard_sabotage_test.go` (mesma isolação de `$HOME`; o gate de
  paridade spawna subprocessos Node/Python que herdam a env var).
- `internal/generators/credential_guard_dedup_test.go` (novo — 9 testes: 6 cenários "global instalado
  → entrada por-projeto pulada, attention-signal/cleanup preservados" um por CLI, + 3 fail-open:
  sem arquivo global, JSON corrompido, arquivo sem permissão de leitura).
- `npm/src/generators/hooks.js` (mesmo bloco de dedup — `globalCredentialGuardScriptPath`,
  `readGlobalHookJSON`, `hookArrayHasCommand`, `globalCredentialGuardInstalled<Tool>` — e os 6
  `injectXHooks`).
- `npm/tests/generators.test.js` (hook `test.beforeEach`/`test.afterEach` a nível de arquivo isolando
  `$HOME` para todos os testes do arquivo).
- `npm/tests/credential_guard_sabotage.test.js` (isolação de `$HOME` em `setupSabotageFixture`).
- `npm/tests/credential_guard_dedup.test.js` (novo — 8 testes espelhando os do Go).
- `pypi/trackfw/generators/hooks.py` (mesmo bloco de dedup —
  `_global_credential_guard_script_path`, `_read_global_hook_json`, `_hook_array_has_command`,
  `_simple_array_has_value`, `_global_credential_guard_installed_<tool>` — e as 6
  `inject_x_hooks`).
- `pypi/tests/test_generators_init.py` (`TestAttentionHooksInjectors.setUp`/`tearDown` isolando
  `$HOME`).
- `pypi/tests/test_credential_guard_sabotage.py` (`SabotageFixtureMixin.setUp`/`tearDown` isolando
  `$HOME`).
- `pypi/tests/test_credential_guard_dedup.py` (novo — 8 testes espelhando os do Go/Node).

**Evidência de validação:**
- `go build ./... && go vet ./... && go test ./...` — todos os pacotes verdes.
- `cd npm && npm test` — 433 testes verdes (425 pré-existentes + 8 novos de dedup).
- `python3 -m pytest pypi/` — 972 testes verdes + 8 subtests (964 pré-existentes + 8 novos).
- `GO_BIN=bin/trackfw scripts/check-agent-hooks-parity.sh` — todos os 12 cenários (6 CLIs × Go-vs-
  Node/Go-vs-Python) OK, sem regressão (cenário padrão sem global instalado — o mesmo que o gate já
  cobria antes deste ML).

Nenhum commit feito por este agente (git authority é do `trackfw_architect`). Próxima wave é a
Wave 4 (`ML-4A` — estender o gate de paridade estrutural para os alvos harness), ainda
`⬜ Pendente`.
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
**Status:** ✅ Concluído

**Gate novo (não extensão):** `scripts/check-harness-hooks-parity.sh` — script dedicado seguindo
exatamente o padrão de `check-agent-hooks-parity.sh` (mesma estrutura: resolução de runtimes,
guards de vacuidade P2, comparador `python3` inline sem `jq`), mas para os 6 alvos globais
(`<tool>-credential-guard` via `trackfw update harness --targets ... --install-missing`) em vez do
`discover --init` por-projeto. Não estendeu o script existente porque os dois exercitam entry points
e fixtures (`$HOME` isolado vs. projeto) completamente diferentes — misturá-los violaria a separação
de responsabilidade já estabelecida pelo gate original.
**Normalização do path absoluto:** cada um dos 6 arquivos embute o path ABSOLUTO de
`~/.trackfw/scripts/trackfw-credential-guard.sh` (correto — um hook global precisa resolver a partir
de qualquer projeto). Como cada runtime roda contra seu PRÓPRIO `$HOME` de fixture isolado (rejeitada
a opção de `$HOME` compartilhado entre os 3 runtimes na mesma rodada — `--install-missing` é merge
idempotente, então o 2º/3º runtime a escrever no mesmo `$HOME` reportaria `state: skipped` em vez de
`state: updated`, mascarando o comportamento real de escrita-do-zero de cada stack), o gate substitui
textualmente o path do `$HOME` de fixture de cada runtime por um placeholder comum (`<HOME>`) no
conteúdo bruto do arquivo antes de fazer `json.loads` — normalização puramente textual, não
regex/glob, então nunca falso-nega um path absoluto realmente divergente por outro motivo.
**Achado durante implementação:** `mktemp -d "${TMPDIR:-/tmp}/..."` no macOS produzia um `$WORK` com
barra dupla (`TMPDIR` já termina em `/`), que os 3 runtimes normalizam ao montar o path absoluto
embutido (`filepath.Join`/`path.join`/`os.path.join`) mas que o `$HOME` literal usado pela
normalização textual do gate NÃO normalizava — causando falso-positivo de drift em todo campo com o
path embutido. Corrigido canonicalizando `$WORK` uma vez (`WORK=$(cd "$WORK" && pwd)`) logo após o
`mktemp -d`.
**Resultado do gate no estado atual:** 12/12 `OK` (6 CLIs × go-vs-node/go-vs-py) —
`GO_BIN=bin/trackfw scripts/check-harness-hooks-parity.sh` verde.
**Arquivos afetados:**
- `scripts/check-harness-hooks-parity.sh` (novo)
- `Makefile` (linha `check-harness-hooks-parity.sh` inserida no alvo `parity`, logo após
  `check-agent-hooks-parity.sh` e antes de `check-gates-falsify.sh`)
- `scripts/check-gates-falsify.sh` (Cenário 45 — corrompe o `matcher` da entrada
  `trackfw-credential-guard-global-post` do wiring global do Kiro no literal Python
  (`pypi/trackfw/commands/update_harness.py`, `"shell"` → `"execute_bash"`) numa cópia isolada do
  repositório; `GO_BIN` real, `NODE_CLI` real via `setup_npm_tree`, `PY_ROOT` corrompido; asserta
  reprovação com `$.hooks[1].matcher` no diagnóstico; contagem final atualizada de 102→103 cenários,
  17→18 gates)
- `docs/cli-parity.md` (nova seção "Hooks GLOBAIS de credential-guard ... — paridade estrutural
  (ROADMAP-2026-08-06, ML-4A)")
**Ações:**
- Mesmo padrão do `check-agent-hooks-parity.sh` (PR #141), mas com fixture de `$HOME` isolado em vez
  de projeto — gerar os 6 alvos globais via Go/Node/Python reais e comparar estruturalmente.
**Critérios de aceite:**
- [x] Gate novo/estendido verde para os 6 alvos
- [x] Prova negativa registrada em `check-gates-falsify.sh`
**Comandos de validação:** `make quality`

Nenhum commit feito por este agente (git authority é do `trackfw_architect`). Próxima wave é a
Wave 5 (`ML-5A` — consolidar documentação e fechar REQ), ainda `⬜ Pendente`.

## Wave 5 — Documentação e encerramento (1 ML)
> Dependências: Waves 1-4 completas

### ML-5A — Consolidar documentação e fechar REQ
**Status:** ✅ Concluído

**Nota de auditoria:** `docs/cli-parity.md` ganhou seção consolidada de escopo global (tabela por CLI
+ achados transversais das Waves 1-4). O agente encontrou e reportou uma colisão de título real: já
existia uma seção "Suporte por CLI — visão consolidada (ML-5A)" do roadmap anterior (PR #141, escopo
de projeto) com o mesmo rótulo `ML-5A`. Corrigido diretamente pelo orquestrador — título antigo
renomeado para deixar explícito "escopo DE PROJETO" com nota cruzada apontando para a seção nova
("escopo GLOBAL"), sem reescrever o conteúdo técnico da seção antiga. REQ atualizada com os 7
Acceptance Criteria concluídos (100%, diferente dos dois ciclos anteriores que fecharam com
ressalvas) — `make quality` confirmado passando de ponta a ponta. ADR movida de `Proposed` para
`Accepted`. Frontmatter `roadmap:` da REQ, que ficou vazio desde a criação (não sincronizou),
corrigido junto.
**Arquivos afetados:**
- `docs/cli-parity.md`, `docs/agents-working-context.md`, REQ, ADR
**Ações:**
- Documentar os 6 alvos novos, a decisão de dedup, e as duas investigações resolvidas (Codex,
  Kiro v3) numa seção consolidada.
- Atualizar REQ (Acceptance Criteria + Linked Roadmap).
**Critérios de aceite:**
- [x] `trackfw validate`/`make quality` sem regressão
**Comandos de validação:** `trackfw validate && make quality`
