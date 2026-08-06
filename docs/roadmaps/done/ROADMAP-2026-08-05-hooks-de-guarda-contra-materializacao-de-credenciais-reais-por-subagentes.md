---
status: done
date: 2026-08-05
req: "docs/req/REQ-2026-08-05-hooks-de-guarda-contra-materializacao-de-credenciais-reais-por-subagentes.md"
squad: ""
---

# Roadmap: hook de guarda contra materialização de credenciais reais por subagentes

> Created: 2026-08-05 | Status: done

## Context
REQ: `docs/req/REQ-2026-08-05-hooks-de-guarda-contra-materializacao-de-credenciais-reais-por-subagentes.md`

Achado em `ea-cmdb` (projeto consumidor do trackfw): subagentes especialistas (QA, backend)
materializaram JWTs reais em texto plano (arquivos soltos + stdout) ao validar endpoints
autenticados "com evidência real". Nenhum gate de pre-commit pega esse padrão porque audita só o
que está staged.

Decisão registrada em `ADR-2026-08-05-hook-de-guarda-contra-materializacao-de-credenciais-reais-por-subagentes.md`:

1. Novo hook `trackfw-credential-guard.sh`, modo **avisador por padrão** (bloqueio opt-in via
   `trackfw.yaml: credential_guard.mode: warn|block`).
2. Wave nativa cobre os **6 CLIs com algum hook pré/pós-execução hoje suportado pelo trackfw**:
   Claude Code, Codex, Gemini CLI, GitHub Copilot, Cursor, Kiro. Windsurf fica fora (sem hook nativo
   pré-execução — confirmado por `REQ-2026-06-20-attention-hooks-agent-clis.md` e comentário já
   existente em `agentfiles.go`).
3. Paridade Go/Node/Python obrigatória desde o primeiro commit de cada ML.
4. Gate de paridade existente (`scripts/check-attention-scripts-parity.sh`) só cobre os 2 scripts
   shell — precisa de extensão para cobrir os `hooks.json`/`settings.json` por CLI.
5. Teste de sabotagem obrigatório (materializa JWT sintético de fato, não reimplementa a checagem em
   paralelo).

Mecanismo já existente a reaproveitar: `internal/generators/hooks.go:InjectHooksDetected`
(dispatcher) + `internal/generators/agentfiles.go:InjectXHooks` por CLI (linhas 182–437) +
`internal/generators/scaffold.go:GenerateAttentionScripts` (linha 686, gera os scripts shell
embutidos) — com paridade em `npm/src/generators/hooks.js` e `pypi/trackfw/generators/hooks.py`.

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ] Roadmap executado com decisão explícita já registrada na ADR (bloqueante vs. avisador:
      avisador por padrão, bloqueio opt-in via `trackfw.yaml`)
- [ ] Script de guarda com paridade Go/Node/Python confirmada desde o primeiro commit
- [ ] Claude Code coberto na Wave 2 (`PreToolUse`/`PostToolUse` em `Bash`); demais CLIs da wave
      nativa (Codex, Gemini, Copilot, Cursor, Kiro) cobertos ou re-escopados com motivo documentado
- [ ] Teste de sabotagem real (materializa JWT sintético, invoca o script como subprocesso) — não um
      self-test que reimplementa a checagem em paralelo
- [ ] Gate de paridade estendido para cobrir os `hooks.json`/`settings.json` por CLI
- [ ] `make quality`/`make parity` verdes, `trackfw validate` sem violações novas

## Wave 1 — Script de guarda + config (1 ML)
> Dependências: Independente

### ML-1A — Script `trackfw-credential-guard.sh` + campo de config `credential_guard.mode`
**Status:** ✅ Concluído
**Arquivos afetados (reais):**
- `internal/generators/scaffold.go` (`GenerateCredentialGuardScript` + const `credentialGuardScript`)
- `internal/generators/credential_guard_test.go` (novo — geração, paridade cross-stack, 11 cenários
  de comportamento via subprocess real)
- `internal/config/config.go` (`CredentialGuardConfig{Mode string}`, default `"warn"`) +
  `internal/config/config_test.go`
- `npm/src/generators/hooks.js` (`CREDENTIAL_GUARD_SCRIPT` + `generateCredentialGuardScript`) —
  localização real diferente do previsto (`hooks.js`, não `scaffold.js`, por ser onde a função irmã
  de attention-hooks já vive)
- `npm/src/config/index.js` + `npm/tests/credential_guard.test.js` (novo)
- `pypi/trackfw/generators/init_gen.py` (`_CREDENTIAL_GUARD_SH` + `_generate_credential_guard_script`)
  — localização real diferente do previsto (`init_gen.py`, não `generators/scaffold.py`)
- `pypi/trackfw/config.py` + `pypi/tests/test_credential_guard.py` (novo)
- `docs/cli-parity.md` (nova seção documentando `credential_guard.mode`)

**Decisões de design registradas na auditoria (não 100% especificadas no ML original):**
- Shebang `#!/usr/bin/env bash` (não POSIX `sh`) — alinhado aos scripts irmãos de attention-hook,
  que já usam bash-only features.
- Exceção de destino efêmero é mais estrita que "contém mktemp/dev-null": só isenta quando **todos**
  os alvos de redirect são `/dev/null`, um `$(mktemp...)` direto, ou variável atribuída via
  `VAR=$(mktemp...)`; match sem redirecionamento (stdout) ou redirecionado a caminho comum sempre
  alerta.
- Valor de `mode` inválido cai silenciosamente para `warn` nos 3 stacks — mesmo padrão de outros
  campos de formato não reconhecido no parser (`roadmap_namespacing`, `forge`).
- Formato do JSON de attention é `{tool, message, level, timestamp}` — espelha o que
  `trackfw-attention-signal.sh` realmente escreve (não o schema `{roadmap, ml, ...}` documentado em
  `CLAUDE.md` para sinalização autoral de agente).

**Limitação conhecida — Resolvida (fix aplicado após a auditoria do ML-2B):**
`trackfw-attention-cleanup.sh` em `PostToolUse` apagava o mesmo `.trackfw-attention.json` que este
hook escrevia em modo `warn`. A auditoria do ML-2A (Claude Code) tinha concluído que não havia race
real ali, porque os matchers `AskUserQuestion`/`Bash` são mutuamente exclusivos nesse CLI. A
investigação de concorrência do ML-2B (Codex), porém, confirmou contra a documentação oficial do
Codex CLI (<https://developers.openai.com/codex/hooks>) que hooks do mesmo evento com matchers
diferentes batendo no mesmo `tool_name` **rodam concorrentemente** ("Multiple matching command hooks
for the same event are launched concurrently") — no wiring do Codex, `PostToolUse[".*"]` (cleanup) e
`PostToolUse["Bash"]` (credential-guard) colidem numa mesma chamada Bash, então o `rm -f` do cleanup
podia de fato apagar o aviso do credential-guard escrito na mesma invocação. Corrigido decouplando o
credential-guard do arquivo compartilhado: o modo `warn` agora escreve em
`$ROADMAP_DIR/.trackfw-credential-guard.json`, um arquivo dedicado que nenhum outro script gerado
toca — elimina a race por completo, independente do modelo de concorrência de cada CLI. Ver
`docs/cli-parity.md` (seção `credential_guard.mode`) e
`internal/generators/credential_guard_test.go:TestCredentialGuardScript_AttentionCleanupDoesNotDeleteIt`
(+ equivalentes Node/Python).

**Achado paralelo, fora de escopo:** `make parity` falha por divergência pré-existente de versão
(`pypi/trackfw/__init__.py` com fallback `6.3.1` vs. `6.4.1` em Go/Node) — confirmado não relacionado
a este ML (`git stash` + rerun reproduz a mesma falha sem as mudanças). Não corrigido aqui; registrar
como achado separado antes do release.
**Ações:**
- Escrever o script `trackfw-credential-guard.sh` (POSIX sh, sem dependências externas) que:
  - Lê `tool_input.command` (para `PreToolUse`) ou a saída do comando (para `PostToolUse`) via
    stdin JSON (mesmo padrão de parsing já usado em `trackfw-attention-signal.sh` — reusar a lógica
    de leitura de JSON, não reinventar).
  - Aplica regex de JWT: `eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`.
  - Aplica regex de AWS key: `AKIA[0-9A-Z]{16}`.
  - Ignora destinos efêmeros: comandos de `echo`/`cat`/`>` para `mktemp`/`/dev/null` não disparam
    alerta.
  - Lê `credential_guard.mode` de `trackfw.yaml` (grep simples, sem parser YAML completo — mesmo
    approach leve já usado nos outros scripts shell do projeto); default `warn` se ausente.
  - Modo `warn`: escreve aviso em stderr + loga em `docs/roadmaps/.trackfw-attention.json`
    (`level: "action_required"`, mensagem descrevendo o padrão detectado) e sai com código 0.
  - Modo `block` (só no `PreToolUse`): sai com código 2 (convenção de bloqueio do Claude Code/Codex/
    Kiro — confirmar comportamento equivalente por CLI na Wave 2).
- Adicionar `CredentialGuardMode string \`yaml:"mode"\`` dentro de uma struct `CredentialGuard` no
  schema de config Go, Node e Python, com default `"warn"` nos 3 stacks.
- Documentar a nova chave em `docs/cli-parity.md` (seção de `trackfw.yaml`).
**Critérios de aceite:**
- [ ] `go build ./...` sem erros
- [ ] `npm run test --workspace=npm` e `python -m pytest pypi/` verdes
- [ ] Script novo idêntico em intenção (não byte-a-byte — cada stack pode ter estilo de shell
      próprio, mas o `docs/cli-parity.md` deve descrever o contrato comum) nos 3 stacks
- [ ] `trackfw.yaml` de exemplo do próprio repo aceita a nova chave sem quebrar `trackfw validate`
**Comandos de validação:** `go build ./... && make test && make lint`

## Wave 2 — Wiring por CLI (6 MLs em paralelo)
> Dependências: Wave 1 completa (script e config precisam existir antes de referenciá-los)

Cada ML abaixo toca os 3 stacks (Go/Node/Python) do CLI correspondente na mesma unidade de trabalho,
para não repetir o erro de `REQ-2026-08-04-scripts-de-attention-hooks-divergem-...` (paridade
quebrada por lote parcial).

### ML-2A — Claude Code
**Status:** ✅ Concluído

**Nota de auditoria:** confirmado que não há race real no `PostToolUse` — os matchers `AskUserQuestion`
e `Bash` são mutuamente exclusivos por `tool_name` dentro do mesmo evento (`trackfw-attention-cleanup.sh`
não lê `tool_name`, mas só é invocado pelo Claude Code quando o matcher bate); nenhum ajuste de
ordenação foi necessário. Efeito colateral positivo: corrigido bug pré-existente em
`mergeClaudeHookArray` (Go) que criava um bloco `{"matcher":...}` duplicado em vez de mesclar no
array `hooks` existente quando o mesmo matcher já tinha outro comando — agora em paridade com o
comportamento que Node já tinha; Python ganhou o helper equivalente.
**Arquivos afetados:**
- `internal/generators/agentfiles.go` (`InjectClaudeHooks`, linha 182-227; usar
  `mergeClaudeHookArray`, linha 441, para adicionar entradas sem sobrescrever hooks existentes de
  terceiros)
- `npm/src/generators/hooks.js` (linha ~142, função irmã)
- `pypi/trackfw/generators/hooks.py` (linha ~43, função irmã)
**Ações:**
- Adicionar ao array `PreToolUse` de `.claude/settings.json`: `{matcher:"Bash", hooks:[{type:"command",
  command:"scripts/trackfw-credential-guard.sh"}]}`.
- Adicionar a mesma entrada em `PostToolUse` com matcher `"Bash"`.
- Usar `mergeClaudeHookArray`/equivalentes para não sobrescrever a entrada existente de
  `AskUserQuestion` — os dois hooks (attention-signal e credential-guard) devem coexistir no mesmo
  array `PreToolUse`/`PostToolUse`.
**Critérios de aceite:**
- [ ] `trackfw init`/`trackfw discover` num projeto de teste gera `.claude/settings.json` com AMBOS
      os hooks (`AskUserQuestion` preservado + `Bash` novo)
- [ ] Rodar novamente (`update`) é idempotente — não duplica entradas
- [ ] Testes existentes de `agentfiles_test.go` (Go), equivalentes Node/Python, continuam verdes
**Comandos de validação:** `go test ./internal/generators/... && npm run test --workspace=npm -- hooks && python -m pytest pypi/tests/ -k hooks`

### ML-2B — Codex
**Status:** ✅ Concluído

**Nota de auditoria:** confirmado via `https://developers.openai.com/codex/hooks` (2026-08-05) que
`PreToolUse[matcher:"Bash"]` intercepta todo tool-call de Bash, distinto de `PermissionRequest`
(que só dispara quando Codex vai pedir aprovação — não para todo comando). Divergência da pesquisa
preliminar da ADR: hooks vêm **habilitados por padrão** no Codex CLI atual — `[features] hooks`
(alias legado `codex_hooks`) existe para **desabilitar**, não para opt-in; nenhuma injeção de
`config.toml` foi necessária. Bloqueio via `PreToolUse` usa exit code 2 + stderr, já compatível com
o modo `block` existente do script. Efeito colateral: o merge do Python (`inject_codex_hooks`) só
checava presença do comando em qualquer lugar do array — não mesclava num matcher já existente como
Go/Node já faziam; corrigido com o novo helper `_merge_codex_hook_entry` para trazer paridade real
de comportamento de merge entre os 3 stacks. Detalhe completo em `docs/cli-parity.md` (seção "Codex
wiring (ML-2B)"). **Achado adicional desta auditoria, corrigido fora deste ML:** a mesma citação da
documentação do Codex sobre hooks concorrentes ("Multiple matching command hooks for the same event
are launched concurrently") revelou que a "Limitação conhecida" registrada no ML-1A não era teórica —
neste wiring especificamente, `PostToolUse[".*"]` (cleanup) e `PostToolUse["Bash"]`
(credential-guard) colidem na mesma chamada Bash e rodam em paralelo, permitindo que o `rm -f` do
cleanup apagasse o aviso do credential-guard escrito na mesma invocação. O wiring do Codex em si
(matchers, merge, formato do `hooks.json`) está correto e não precisou de nenhuma mudança; o fix foi
aplicado no conteúdo do script (`.trackfw-credential-guard.json` dedicado) e está registrado na seção
do ML-1A acima.
**Arquivos afetados:**
- `internal/generators/agentfiles.go` (`InjectCodexHooks`, linha 230-276)
- `internal/generators/agentfiles_test.go` (`TestInjectCodexHooks` estendido +
  `TestInjectCodexHooks_PreservesExistingBashEntry` novo)
- `internal/generators/codex_test.go` (`TestInstallCodexCreatesNativeArtifacts` estendido)
- `npm/src/generators/hooks.js` (linha ~157)
- `npm/tests/generators.test.js` (`injectCodexHooks` — asserts estendidos + teste de merge novo)
- `npm/tests/codex.test.js` (`installCodex` — asserts atualizados para `PreToolUse`/`PostToolUse[Bash]`)
- `pypi/trackfw/generators/hooks.py` (`inject_codex_hooks` + novo helper `_merge_codex_hook_entry`)
- `pypi/tests/test_generators_init.py` (`test_inject_codex_hooks_create_and_merge` estendido +
  `test_inject_codex_hooks_preserves_existing_bash_entry` novo)
- `pypi/tests/test_codex.py` (`test_install_codex_creates_idempotent_native_artifacts` — asserts
  atualizados)
- `docs/cli-parity.md` (nova seção "Codex wiring (ML-2B)" com fonte da investigação)
**Ações:**
- Investigar primeiro (não assumir): o Codex expõe `PreToolUse` real com matcher dedicado a `Bash`
  (conforme docs oficiais pesquisadas: "PreToolUse intercepta o shell (Bash) tool only — by design"),
  distinto do `PermissionRequest` já usado hoje pelo trackfw para o attention-signal. Confirmar em
  `.codex/config.toml` se `[features] codex_hooks = true` precisa ser injetado também (feature é
  experimental conforme doc pesquisada — versão mínima do Codex a exigir deve ser documentada no
  commit).
- Adicionar `PreToolUse[matcher:"Bash"]` e `PostToolUse[matcher:"Bash"]` (ou matcher equivalente
  confirmado na investigação) apontando para `scripts/trackfw-credential-guard.sh`, preservando a
  entrada existente de `PermissionRequest` para o attention-signal.
**Critérios de aceite:**
- [ ] Formato final documentado em `docs/cli-parity.md` com a fonte da investigação (versão mínima
      do Codex, flag de feature necessária)
- [ ] `.codex/hooks.json` gerado contém ambos os hooks sem sobrescrever o existente
- [ ] Testes de geração verdes nos 3 stacks
**Comandos de validação:** `go test ./internal/generators/... && npm run test --workspace=npm -- hooks && python -m pytest pypi/tests/ -k hooks`

### ML-2C — Gemini CLI
**Status:** ✅ Concluído

**Nota de auditoria:** confirmado via `https://geminicli.com/docs/hooks/reference` (2026-08-05, texto
extraído por `curl` — sem acesso a WebFetch/WebSearch nesta execução) que existe um evento `BeforeTool`
distinto de `Notification[ToolPermission]`: "Fires before a tool is invoked. Used for argument
validation, security checks, and parameter rewriting" e suporta `Exit Code 2 (Block Tool): Prevents
execution. Uses stderr as the reason` — compatível com o modo `block` já existente do script. O nome
canônico do tool de shell é `run_shell_command` (doc: "you can match any built-in tool (for example,
`read_file`, `run_shell_command`)"); o `matcher` é regex avaliado contra `tool_name`. `AfterTool` já
usado hoje pelo trackfw é de fato o evento pós-execução equivalente a `PostToolUse` ("Fires after a
tool executes"). Implementado `BeforeTool[matcher:"run_shell_command"]` +
`AfterTool[matcher:"run_shell_command"]` apontando para `trackfw-credential-guard.sh`, preservando
`Notification[ToolPermission]` (signal) e `AfterTool[matcher:"*"]` (cleanup) existentes como entradas
separadas no mesmo array.

**Concorrência (item explicitamente pedido na investigação):** o schema documentado tem um campo
`sequential` por grupo de matcher ("If true, hooks in this group run one after another. If false, they
run in parallel"), mas isso só ordena hooks **dentro do mesmo grupo/matcher** — a doc não especifica o
modelo de concorrência **entre grupos diferentes** que batem no mesmo evento e no mesmo `tool_name`
(ex.: `AfterTool["*"]` vs. `AfterTool["run_shell_command"]`, ambos disparando para uma chamada de
shell). Não assumido nenhum modelo (nem serial nem paralelo) por falta de confirmação documental —
registrado como tal no código (`agentfiles.go`) e aqui. Isso não é bloqueante porque o fix do ML-1A
(modo `warn` do credential-guard escreve em `.trackfw-credential-guard.json`, arquivo dedicado que
nenhum outro script gerado toca) já neutraliza a race de "cleanup apaga o warn" independente do modelo
de concorrência real do Gemini CLI — a mesma lógica que resolveu a race confirmada para o Codex no
ML-2B se aplica aqui sem mudança adicional. Nenhum outro efeito colateral de concorrência foi
identificado (nenhum outro script gerado lê/escreve o arquivo dedicado do credential-guard; não há
limite documentado de hooks concorrentes por evento no Gemini CLI que descartaria um deles).

**Efeito colateral, Python:** `inject_gemini_hooks` usava checagem inline ("existe uma entrada com esse
comando em algum lugar do array?") em vez do helper compartilhado `_merge_claude_hook_array` (já usado
por `inject_claude_hooks` no mesmo arquivo) — o mesmo padrão de divergência que o ML-2A corrigiu no Go
(`mergeClaudeHookArray`) e o ML-2B corrigiu no Python para o Codex (`_merge_codex_hook_entry`).
Reescrito para usar `_merge_claude_hook_array` nos 3 grupos (`Notification`, `BeforeTool`, `AfterTool`),
trazendo paridade real de comportamento de merge com Go/Node. Efeito colateral dessa escolha: os campos
`name`/`timeout: 10000` que a versão anterior do Python escrevia nas entradas do Gemini (e que Go/Node
nunca escreveram) foram removidos, para que a saída estruturada fique idêntica entre os 3 stacks
(nenhum teste dependia desses campos).

**Achados fora de escopo, reportados e não corrigidos neste ML:**
- `GenerateCredentialGuardScript`/`generateCredentialGuardScript`/`_generate_credential_guard_script`
  (que escrevem `scripts/trackfw-credential-guard.sh` em disco) não eram chamados por nenhum fluxo de
  comando real (`trackfw init`/`discover`/`update`) nos 3 stacks — apenas por testes. Todo o wiring de
  hooks feito em ML-2A/2B/2C apontava para um script que só existia se algo o gerasse manualmente.
  **Resolvido em commit dedicado logo após este ML** (bug crítico, corrigido antes de prosseguir para
  ML-2D): chamada adicionada ao lado de `GenerateAttentionScripts`/equivalentes em todos os pontos
  reais (Go: `scaffold.go:Scaffold`, `update.go:Update` + `runProjectTarget("agent-hooks")`,
  `discover.go:InstallGates`; Node: `init.js:scaffold`, `discover.js`, `update.js`; Python:
  `hooks.py:inject_hooks_detected` + `init_gen.py:scaffold` + `discover.py`), incondicional (mesma
  condição do gerador irmão), com testes de fluxo real (não só chamada direta do gerador) e cenário de
  upgrade (`update` num projeto pré-existente sem o script) cobertos nos 3 stacks. Confirmado
  end-to-end pelo orquestrador: `trackfw init` num diretório novo gera o script executável e o wiring
  com matcher `Bash` no `.claude/settings.json` gerado.
- `AfterTool[matcher:"*"]` (entrada pré-existente de cleanup, não tocada neste ML): a documentação
  pesquisada não define semântica explícita de "match-all" para o `matcher` (é descrito como regex ou
  string exata; nenhuma menção a `"*"` como coringa documentado). Não corrigido aqui (fora do escopo
  pedido — preservar a entrada existente), mas registrado como suspeita a investigar antes do ML-5A:
  se `"*"` não for tratado como wildcard pelo Gemini CLI real, o cleanup nunca dispara via `AfterTool`.

**Arquivos afetados:**
- `internal/generators/agentfiles.go` (`InjectGeminiHooks`, linha 279-324)
- `npm/src/generators/hooks.js` (linha ~172)
- `pypi/trackfw/generators/hooks.py` (linha ~133)
**Ações:**
- Investigar se existe evento `BeforeTool` genérico (citado na doc pública pesquisada,
  `geminicli.com/docs/hooks/reference`) que intercepta antes da execução real do comando, distinto do
  `Notification[ToolPermission]` já usado hoje. Matcher para tool events no Gemini é regex — usar
  algo como `matcher:"run_shell_command"` ou nome real do tool de shell do Gemini CLI (confirmar
  nome exato do tool na investigação, não assumir).
- Adicionar `BeforeTool`/`AfterTool` com o matcher confirmado, preservando `Notification[ToolPermission]`
  existente.
**Critérios de aceite:**
- [x] Nome exato do tool de shell do Gemini CLI documentado em `docs/cli-parity.md` com a fonte
- [x] `.gemini/settings.json` gerado contém ambos os hooks sem sobrescrever o existente
- [x] Testes de geração verdes nos 3 stacks
**Comandos de validação:** `go test ./internal/generators/... && npm run test --workspace=npm -- hooks && python -m pytest pypi/tests/ -k hooks`

### ML-2D — GitHub Copilot
**Status:** ✅ Concluído

**Nota de auditoria:** confirmado via `https://docs.github.com/en/copilot/reference/hooks-reference`
(2026-08-05, texto extraído via `curl` do JSON `renderedPage` embutido pelo Next.js, sem acesso a
WebFetch/WebSearch nesta execução) que o formato real de `.github/hooks/*.json` é
`{"version": 1, "hooks": {"<event>": [{"type": "command", "bash": "...", "cwd": "...",
"timeoutSec": N}, ...]}}` — exatamente o formato que **Python já usava**. O formato
`{"hooks": [{"event", "run"}]}` que Go e Node emitiam não corresponde a nenhum formato documentado
pelo GitHub; Go e Node foram alinhados ao formato do Python (que estava correto) neste ML. Confirmado
também que existe suporte real a `matcher` (regex ancorado `^(?:PATTERN)$` testado contra `toolName`)
em `preToolUse`/`postToolUse`, ao contrário do pressuposto inicial do ADR — usado `matcher: "bash"`
(nome runtime do tool de shell, minúsculo, válido para eventos em camelCase como os usados aqui;
`"Bash"` maiúsculo só se aplica a eventos PascalCase/payload formato VS Code). O script
`trackfw-credential-guard.sh` foi inspecionado e não depende de nenhum nome de campo específico do
payload (faz grep sobre o payload bruto inteiro) — a escolha de casing/matcher afeta só a precisão do
escopo, nunca a detecção. Concorrência: "If multiple hooks of the same type are configured, they
execute in order" — resposta mais definitiva entre todos os CLIs cobertos até aqui (serial, em ordem
de configuração), ao contrário do modelo concorrente confirmado do Codex (ML-2B) ou do modelo
indocumentado do Gemini (ML-2C). Arquivo dedicado (overwrite total, mesmo padrão do Kiro) — sem
necessidade de merge helper. Detalhe completo, incluindo a citação da tabela de campos que omite
`matcher` (tratado defensivamente, não como bloqueio) e a nota de fail-closed do `preToolUse` em erro
não-zero, em `docs/cli-parity.md` (seção "GitHub Copilot wiring (ML-2D)").
**Arquivos afetados:**
- `internal/generators/agentfiles.go` (`InjectCopilotHooks`, linha ~436-511 — formato realinhado ao
  de Python + novas entradas `matcher:"bash"` para credential-guard)
- `internal/generators/agentfiles_test.go` (`TestInjectCopilotHooks` — asserts reescritos para o novo
  formato `{version, hooks:{preToolUse:[...], postToolUse:[...]}}`)
- `internal/generators/copilot_hooks_parity_test.go` (novo — `TestInjectCopilotHooks_StructuralParityAcrossStacks`,
  invoca Go/Node/Python como subprocessos reais e compara a estrutura JSON resultante)
- `npm/src/generators/hooks.js` (`injectCopilotHooks`, linha ~365 — formato realinhado + novas
  entradas)
- `npm/tests/generators.test.js` (`injectCopilotHooks` — asserts reescritos)
- `pypi/trackfw/generators/hooks.py` (`inject_copilot_hooks`, linha ~238 — novas entradas
  `matcher:"bash"` de credential-guard; formato pré-existente mantido, era o correto)
- `pypi/tests/test_generators_init.py` (`test_inject_copilot_hooks` — asserts estendidos)
- `docs/cli-parity.md` (nova seção "GitHub Copilot wiring (ML-2D)")
**Ações:**
- Confirmar o formato real de `.github/hooks/hooks.json` do Copilot (a doc pesquisada usa
  `preToolUse`/`postToolUse` como chaves de nível superior — determinar qual dos dois formatos hoje
  gerados pelos stacks está correto e alinhar os 3 antes de adicionar o novo hook). ✅
- O formato atual do trackfw para Copilot não tem campo de matcher por tool — o filtro para "só
  Bash" precisa acontecer dentro do próprio `trackfw-credential-guard.sh` inspecionando o payload
  recebido via stdin (`tool_name`/`tool.name`, confirmar chave exata do payload do Copilot). ✅
  (matcher real existe e foi usado; script já é agnóstico ao payload)
- Adicionar entrada `preToolUse`/`postToolUse` apontando para o script, preservando a existente. ✅
**Critérios de aceite:**
- [x] Divergência de formato Go/Node vs Python corrigida e documentada em `docs/cli-parity.md`
- [x] `.github/hooks/*.json` gerado idêntico em estrutura nos 3 stacks
- [x] Testes de geração verdes nos 3 stacks
**Comandos de validação:** `go test ./internal/generators/... && npm run test --workspace=npm -- hooks && python -m pytest pypi/tests/ -k hooks`

### ML-2E — Cursor
**Status:** ✅ Concluído

**Nota de auditoria:** confirmado via `https://cursor.com/docs/agent/hooks` (2026-08-05, RSC payload
extraído por `curl -L` e desescapado com `python3`, sem acesso a WebFetch/WebSearch nesta execução):
`beforeShellExecution` é de fato um evento real, distinto e Bash-specific ("Control shell commands"),
com input `{command, cwd, sandbox}` e resposta `{"permission":"allow"|"deny"|"ask","user_message":...,
"agent_message":...}`. `afterShellExecution` também é real — evento pós-execução, apenas auditoria
(sem `permission` de saída). Achado crítico da investigação: o formato hoje usado pelo trackfw
(`preToolUse`/`postToolUse` de nível superior) **não corresponde a nenhum evento documentado** — o
schema real é `{"version":1,"hooks":{"<evento>":[...]}}`, e a lista completa de eventos não inclui
`preToolUse`/`postToolUse` genérico algum. Conforme instrução explícita do ML, essa divergência não foi
migrada aqui (apenas registrada) — a entrada nova foi adicionada em paralelo, sob `hooks.
beforeShellExecution`/`hooks.afterShellExecution`, preservando as entradas legadas intactas.
**Script não precisou de nenhuma mudança**: o contrato "exit 0 = allow (com JSON opcional no stdout,
confirmado por um exemplo mínimo da própria doc que sai 0 sem stdout algum) / exit 2 = deny / outro
código = fail-open" já é exatamente o que `trackfw-credential-guard.sh` faz hoje (`block` → exit 2 +
stderr; `warn` → exit 0, aviso vai para `.trackfw-credential-guard.json` dedicado, não para stdout).
Emitir JSON de resposta estruturado (`user_message`/`agent_message`) para enriquecer a UX do Cursor foi
avaliado e descartado nesta ML: o script é compartilhado byte-a-byte pelos 6 CLIs já cobertos e nenhum
dos outros 5 espera JSON no stdout do guard — a opção mais simples e já 100% compatível com o contrato
documentado é não tocar no script. `matcher` em `beforeShellExecution` existe (regex contra o texto do
comando, não contra `tool_name` — o evento já é shell-specific) mas foi omitido de propósito: o guard
precisa ver todo comando shell, não um subconjunto. Concorrência entre hooks do mesmo evento não foi
documentada na página recuperada (ao contrário de Codex/Copilot); não presumida — não bloqueante aqui,
pois os arrays `beforeShellExecution`/`afterShellExecution` só contêm a entrada única do
credential-guard. Detalhe completo, com citações da doc, em `docs/cli-parity.md` (seção "Cursor wiring
(ML-2E)").
**Achado fora de escopo, não corrigido aqui:** o wiring pré-existente de `preToolUse`/`postToolUse`
(attention-signal/cleanup) usa um formato que não corresponde a evento real do Cursor documentado —
re-escopar essa parte para um evento real (ex.: `stop`/`beforeSubmitPrompt`) fica para uma ML futura.
**Arquivos afetados:**
- `internal/generators/agentfiles.go` (`InjectCursorHooks`, linha 518-599 — merge de
  `hooks.beforeShellExecution`/`hooks.afterShellExecution` adicionado, `preToolUse`/`postToolUse`
  legado preservado sem migração)
- `internal/generators/agentfiles_test.go` (`TestInjectCursorHooks` estendido +
  `TestInjectCursorHooks_PreservesUserVersion` novo)
- `npm/src/generators/hooks.js` (`injectCursorHooks`, linha ~235 — mesma extensão)
- `npm/tests/generators.test.js` (`injectCursorHooks` — asserts estendidos + teste de
  `version` preservado novo)
- `pypi/trackfw/generators/hooks.py` (`inject_cursor_hooks` — mesma extensão)
- `pypi/tests/test_generators_init.py` (`test_inject_cursor_hooks` — asserts estendidos +
  `test_inject_cursor_hooks_preserves_existing_version` novo)
- `docs/cli-parity.md` (nova seção "Cursor wiring (ML-2E)")
**Ações:**
- Migrar (ou adicionar em paralelo) do evento genérico `preToolUse` hoje usado pelo trackfw para o
  evento nativo `beforeShellExecution` (Bash-specific, confirmado por doc oficial pesquisada —
  `docs.cursor.com`/blog GitButler), que já retorna decisão `allow`/`deny`/`ask` — mapear `warn` do
  trackfw para resposta que não bloqueia (permitir + registrar) e `block` para `deny`.
- Preservar a entrada `preToolUse` existente do attention-signal (não migrar essa, só adicionar o
  novo hook em `beforeShellExecution`).
**Critérios de aceite:**
- [x] `.cursor/hooks.json` gerado contém `beforeShellExecution` novo + `preToolUse`/`postToolUse`
      existentes intactos
- [x] Resposta do script mapeada corretamente para o protocolo `allow`/`deny`/`ask` do Cursor
- [x] Testes de geração verdes nos 3 stacks
**Comandos de validação:** `go test ./internal/generators/... && npm run test --workspace=npm -- hooks && python -m pytest pypi/tests/ -k hooks`

### ML-2F — Kiro
**Status:** ✅ Concluído

**Nota de auditoria:** confirmado via `https://kiro.dev/docs/hooks/`, `https://kiro.dev/docs/hooks/types`
e `https://kiro.dev/docs/hooks/actions/` (2026-08-05, RSC/HTML extraído por `curl -L`, sem acesso a
WebFetch/WebSearch nesta execução) que `PreToolUse` ("Before a tool is about to execute", Can block:
**Yes**) é um trigger real e distinto de `PostFileSave`/eventos de IDE — resolve afirmativamente a dúvida
registrada na ADR: o mecanismo de hooks do Kiro de fato intercepta invocações de tool (incluindo shell)
antes da execução, não só eventos de arquivo. Achado crítico da investigação: o wiring pré-existente
(`event`/matcher como objeto `{tool_name:...}`/sem `version`) não corresponde a nenhum campo do schema
documentado — o real é `{"version":"v1","hooks":[{name, trigger, matcher (string regex), action}]}`.
Diferente do precedente do Cursor (ML-2E, arquivo merge com conteúdo de usuário — legado preservado sem
migração), `.kiro/hooks/trackfw-attention.json` é totalmente sobrescrito pelo trackfw (mesmo padrão do
GitHub Copilot, ML-2D) — por isso as entradas legadas `trackfw-attention-signal`/`-cleanup` foram
realinhadas ao schema correto nesta ML (não deixadas quebradas ao lado de entradas novas corretas), em
vez de apenas documentadas. `matcher` para as entradas novas usa `"shell"` (categoria documentada, "all
built-in shell command-related tools"; alternativa ao nome canônico `"execute_bash"`); `.*` (usado antes
para as entradas legadas) não é um valor de matcher documentado — `"*"` (asterisco literal, "all tools")
é o correto e agora usado no lugar. Contrato de bloqueio confirmado mais estrito que Claude
Code/Codex/Gemini: **qualquer** exit code não-zero de um hook `PreToolUse` bloqueia a invocação (não só
exit 2) — `trackfw-credential-guard.sh` foi reauditado e só tem `exit 0`/`exit 2` nos caminhos normais de
operação, então `warn` nunca bloqueia espuriamente no Kiro. Nenhuma mudança no script foi necessária.
Detalhe completo, com citações das 3 páginas, em `docs/cli-parity.md` (seção "Kiro wiring (ML-2F)").
**Arquivos afetados (reais):**
- `internal/generators/agentfiles.go` (`InjectKiroHooks` — schema realinhado + 2 hooks novos de
  credential-guard no mesmo array/arquivo dedicado)
- `internal/generators/agentfiles_test.go` (`TestInjectKiroHooks` — reescrito para o novo schema, 4
  hooks, validação de `trigger`/`matcher` string/ausência de `event`)
- `npm/src/generators/hooks.js` (`injectKiroHooks`, mesma extensão)
- `npm/tests/generators.test.js` (`injectKiroHooks` — asserts reescritos)
- `pypi/trackfw/generators/hooks.py` (`inject_kiro_hooks`, mesma extensão)
- `pypi/tests/test_generators_init.py` (`test_inject_kiro_hooks` — asserts reescritos)
- `docs/cli-parity.md` (nova seção "Kiro wiring (ML-2F)")
**Ações:**
- Investigar se o `PreToolUse`/`tool_name` matcher do Kiro de fato intercepta antes da execução de
  um comando Bash. ✅ Confirmado — evento real, distinto de `PostFileSave`, "Can block: Yes".
- Adicionar `PreToolUse`/`matcher:"shell"` e `PostToolUse`/`matcher:"shell"` apontando para o script,
  no mesmo array `hooks` já reescrito para `trackfw-attention-signal`/`-cleanup` (arquivo dedicado,
  sem entradas de terceiros a preservar). ✅
- Corrigir o schema legado (`event`→`trigger`, matcher objeto→string, `version` ausente→`"v1"`) das
  entradas pré-existentes, já que o arquivo é 100% owned/overwritten pelo trackfw (mesmo padrão do
  GitHub Copilot, ML-2D) — não deixar entradas comprovadamente inválidas ao lado de entradas novas
  corretas no mesmo array. ✅
**Critérios de aceite:**
- [x] Resultado da investigação documentado em `docs/cli-parity.md` (confirmado, com fonte)
- [x] `.kiro/hooks/trackfw-attention.json` gerado com os 4 hooks (signal/cleanup/guard-pre/guard-post),
      idempotente
**Comandos de validação:** `go test ./internal/generators/... && npm run test --workspace=npm -- hooks && python -m pytest pypi/tests/ -k hooks`

## Wave 3 — Extensão do gate de paridade (1 ML)
> Dependências: Wave 2 completa (precisa dos formatos finais de hooks.json por CLI)

### ML-3A — Estender `check-attention-scripts-parity.sh` para cobrir hooks.json por CLI
**Status:** ✅ Concluído

**Nota de auditoria:** criado gate NOVO e independente,
`scripts/check-agent-hooks-parity.sh`, em vez de estender
`check-attention-scripts-parity.sh` in-place (formatos por CLI divergem
legitimamente entre si — comparação estrutural via JSON parseado, não
byte-a-byte como o gate de scripts shell — misturar as duas famílias no mesmo
arquivo teria acoplado dois contratos de comparação diferentes). Roda
`discover --init` UMA vez por runtime (não uma vez por CLI) num fixture que
carrega, de uma vez, o marcador de detecção dos 6 CLIs — isolamento por CLI
foi medido em ~15s a mais em `make quality` sem ganho de detecção real (os
dois guards de vacuidade por CLI cobrem o caso de um detector regredindo
silenciosamente para um CLI). Comparação estrutural via `python3` inline (sem
`jq`: nenhum gate do projeto depende de `jq`, nenhum workflow o instala, e
`python3` já é dependência obrigatória do gate por rodar o CLI Python). **A
primeira execução do gate reprovou de verdade** contra o estado pós-Wave 2:
`_merge_codex_hook_entry` (Python) escrevia campos `timeout`/`statusMessage`
em `.codex/hooks.json` que Go/Node nunca escreveram — divergência
pré-existente do ML-2B, nunca detectada por falta de gate, corrigida aqui
removendo a decoração de Python (mesmo movimento do ML-2C para os campos
`name`/`timeout: 10000` do Gemini). Detalhe completo, incluindo a prova
negativa (Cenário 44 de `check-gates-falsify.sh`), em `docs/cli-parity.md`
(seção "Agent hooks por CLI ... — paridade estrutural (ML-3A)").
**Arquivos afetados (reais):**
- `scripts/check-agent-hooks-parity.sh` (novo — gate independente, não extensão
  de `check-attention-scripts-parity.sh`)
- `Makefile` (alvo `parity` — novo script encadeado após
  `check-attention-scripts-parity.sh`, antes de `check-gates-falsify.sh`)
- `scripts/check-gates-falsify.sh` (Cenário 44 — prova negativa do novo gate;
  contagem de cenários/gates do `echo` final atualizada: 101→102 cenários,
  16→17 gates)
- `pypi/trackfw/generators/hooks.py` (`_merge_codex_hook_entry`/
  `inject_codex_hooks` — remoção da decoração `timeout`/`statusMessage`,
  achado do próprio gate, não pré-planejado neste ML)
- `docs/cli-parity.md` (nova seção "Agent hooks por CLI ... — paridade
  estrutural (ML-3A)")
**Ações:**
- Para cada CLI da Wave 2, gerar o arquivo de hook via Go/Node/Python num diretório temporário e
  comparar **estruturalmente** (chaves presentes, não byte-a-byte, já que os formatos diferem entre
  CLIs mas devem ser idênticos entre os 3 stacks para o mesmo CLI). ✅ (`python3` inline em vez de
  `jq` — decisão registrada em `docs/cli-parity.md`, nenhum gate do projeto depende de `jq`)
- Falhar com mensagem clara indicando qual stack diverge e em qual campo. ✅
**Critérios de aceite:**
- [x] `make quality` (alvo `parity`) roda o novo gate e passa para o estado pós-Wave 2 — confirmado
      individualmente (`scripts/check-agent-hooks-parity.sh` + toda a cadeia de gates até ele, ver
      nota de auditoria); `make parity`/`make quality` completos seguem bloqueados por
      `check-cli-parity.sh` por um achado pré-existente e fora de escopo já registrado no ML-1A
      (`pypi/trackfw/__init__.py` fallback `6.3.1` vs. `6.4.1` em Go/Node) — reconfirmado nesta
      auditoria via `git stash` que o bloqueio existe idêntico sem as mudanças deste ML
- [x] Gate falsifica de propósito (prova negativa, mesmo padrão de `scripts/check-gates-falsify.sh`
      citado em `docs/cli-parity.md`): introduzir divergência manual num stack e confirmar que o
      gate detecta antes de reverter — feito manualmente (Kiro/Node.js, matcher `shell`→
      `execute_bash`) e formalizado como Cenário 44 de `check-gates-falsify.sh`
**Comandos de validação:** `make quality`

## Wave 4 — Teste de sabotagem (1 ML, obrigatório por AC da REQ)
> Dependências: Wave 2 completa (pelo menos ML-2A/Claude Code)

### ML-4A — Teste de sabotagem: materializar JWT sintético e confirmar detecção
**Status:** ✅ Concluído

**Nota de auditoria:** cobertura end-to-end real (wiring gerado via `InjectXHooks` +
payload JSON exato do CLI + script gerado invocado como subprocesso) confirmada para
**3 dos 6 CLIs** — Claude Code (obrigatório pelo AC da REQ), Cursor e Kiro — os únicos
para os quais `docs/cli-parity.md` documenta um exemplo completo e citado do payload
JSON de **stdin em runtime** (não apenas o formato do arquivo de configuração
`hooks.json`/`settings.json`, que é um contrato diferente já coberto por
`agentfiles_test.go`). Codex, Gemini CLI e GitHub Copilot ficaram **sem teste de
sabotagem end-to-end** — motivo explícito, não omissão silenciosa:
- **Codex**: `docs/cli-parity.md` (seção "Codex wiring (ML-2B)") confirma que o
  `matcher` do `hooks.json` é aplicado a `tool_name`, mas o texto recuperado da doc
  oficial (`developers.openai.com/codex/hooks`) não expõe um exemplo completo do
  payload JSON que chega via stdin ao hook em runtime.
- **Gemini CLI**: `docs/cli-parity.md` (seção "Gemini CLI wiring (ML-2C)") confirma o
  nome do evento/matcher (`BeforeTool`/`AfterTool`, `run_shell_command`), mas não há,
  no texto recuperado de `geminicli.com/docs/hooks/reference`, um exemplo de payload
  JSON de stdin para hooks de tool.
- **GitHub Copilot**: `docs/cli-parity.md` (seção "GitHub Copilot wiring (ML-2D)")
  confirma apenas o nome de um campo (`toolName`, formato camelCase) e registra
  explicitamente que o formato de payload depende do casing do nome do evento
  (camelCase vs. PascalCase "VS Code compatible") sem cravar qual dos dois simular —
  reproduzir um payload completo aqui seria inventar um contrato não confirmado.

Construir um payload "por analogia" ao de Claude Code para esses 3 CLIs foi avaliado e
descartado: violaria a instrução explícita do ML de não inventar schema sem confiança
suficiente. `trackfw-credential-guard.sh` já é comprovadamente agnóstico a nomes de
campo (varre o payload bruto inteiro via `grep`), então a ausência de teste aqui é uma
lacuna de **evidência de sabotagem documentada**, não uma lacuna de cobertura de
detecção real — os 3 CLIs sem teste continuam protegidos pelo mesmo script
byte-idêntico validado pelos outros 3.

**Prova de falsificação executada (Claude Code, Go), revertida antes de concluir:**
`internal/generators/scaffold.go`, linhas 819-820 (`JWT_PATTERN`/`AWS_KEY_PATTERN`)
temporariamente trocadas por `'ZZZNEVERMATCHESZZZ'` (neutralizando a detecção sem
quebrar a sintaxe do script). Rerun de
`go test ./internal/generators/... -run TestSabotage_ClaudeCode -v` com a detecção
neutralizada: `TestSabotage_ClaudeCode_JWTInBashCommand_WarnMode` e
`TestSabotage_ClaudeCode_JWTInBashCommand_BlockMode` **falharam** como esperado (warn:
`.trackfw-credential-guard.json` não escrito; block: exit code 0 em vez de 2), enquanto
`TestSabotage_ClaudeCode_WiringReferencesRealScript` e
`TestSabotage_ClaudeCode_NoJWT_ProvaNegativa` continuaram passando (não dependem de
detecção) — confirma que os testes de sabotagem realmente exercitam a lógica de
detecção do script, não um caminho sempre-verde. Alteração revertida
(`git diff --stat internal/generators/scaffold.go` vazio após reversão);
`go test ./internal/generators/... -run TestSabotage -v` volta a passar 100% com o
script original.

**Arquivos afetados (reais):**
- `internal/generators/credential_guard_sabotage_test.go` (novo — Claude Code, Cursor,
  Kiro; 13 testes)
- `npm/tests/credential_guard_sabotage.test.js` (novo — mesmos 3 CLIs; 13 testes,
  `node:test`)
- `pypi/tests/test_credential_guard_sabotage.py` (novo — mesmos 3 CLIs; 13 testes,
  `unittest`)
**Ações:**
- Escrever um teste que, num projeto de fixture com o hook já injetado (Wave 2, Claude Code no
  mínimo), efetivamente invoca o script `trackfw-credential-guard.sh` com um payload contendo um JWT
  sintético (`eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ0ZXN0In0.abc123...`, gerado no próprio teste, nunca um
  token real) simulando o stdin que o Claude Code enviaria em `PreToolUse`.
- Confirmar que o script detecta (saída/exit code/arquivo `.trackfw-attention.json` conforme modo
  `warn`) — **não** reimplementar a regex no teste para comparar consigo mesma (lição de
  `qualidade-selftest-paralelo-falso-verde`); o teste deve invocar o script real como um subprocesso.
- Repetir para os demais CLIs confirmados na Wave 2 (Codex, Gemini, Copilot, Cursor, Kiro) na medida
  em que cada um tiver formato de payload e resposta confirmados; documentar no roadmap quais ficaram
  sem teste de sabotagem por falta de confirmação e por quê (não é falha silenciosa — é status
  explícito).
**Critérios de aceite:**
- [x] Teste de sabotagem para Claude Code passa e falha propositalmente se a detecção do script for
      neutralizada (prova de falsificação documentada acima)
- [x] Cobertura por CLI documentada (quais têm teste de sabotagem, quais não e o motivo)
- [x] `go test ./...`, `npm test`, `python3 -m pytest pypi/` verdes com os novos testes incluídos
**Comandos de validação:** `go test ./... && (cd npm && npm test) && python3 -m pytest pypi/`

## Wave 5 — Documentação e encerramento (1 ML)
> Dependências: Waves 1-4 completas

### ML-5A — Atualizar documentação e contexto de trabalho
**Status:** ✅ Concluído

**Nota de auditoria:** `docs/cli-parity.md` ganhou uma seção consolidada única (tabela por CLI +
achados transversais das Waves 2-4). Durante a consolidação, o agente encontrou e reportou uma
inconsistência real: a seção "Gemini CLI wiring (ML-2C)" ainda descrevia o bug do script nunca gerado
como "não corrigido" — o fix já tinha sido aplicado em commit dedicado logo após aquele ML. Corrigido
diretamente pelo orquestrador (correção factual pontual, não expansão de escopo). REQ atualizada com
4 de 5 Acceptance Criteria concluídos; o 5º (`make quality`/`make parity` totalmente verdes) fica
formalmente aberto, bloqueado por um bug de versão pré-existente e não relacionado a esta REQ
(`pypi/trackfw/__init__.py`), já documentado desde o ML-1A — `trackfw validate` passa limpo e todos
os gates específicos desta REQ foram confirmados verdes isoladamente.
**Arquivos afetados:**
- `docs/cli-parity.md`
- `docs/agents-working-context.md`
- `docs/req/REQ-2026-08-05-hooks-de-guarda-contra-materializacao-de-credenciais-reais-por-subagentes.md`
  (marcar Acceptance Criteria concluídos, preencher `## Linked Roadmap`)
**Ações:**
- Consolidar em `docs/cli-parity.md` a tabela final de suporte por CLI (incluindo quaisquer
  limitações descobertas nas Waves 2-4, ex.: Kiro re-escopado, Windsurf fora).
- Atualizar `docs/agents-working-context.md` com o resumo do ciclo.
**Critérios de aceite:**
- [ ] `trackfw validate` sem violações novas
- [ ] `make quality` verde
**Comandos de validação:** `trackfw validate && make quality`
