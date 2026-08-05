---
status: wip
date: 2026-08-05
req: "docs/req/REQ-2026-08-05-hooks-de-guarda-contra-materializacao-de-credenciais-reais-por-subagentes.md"
squad: ""
---

# Roadmap: hook de guarda contra materialização de credenciais reais por subagentes

> Created: 2026-08-05 | Status: wip

## Diagnóstico / Contexto

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

## Wave 1 — Script de guarda + config (1 ML)
> Dependências: Independente

### ML-1A — Script `trackfw-credential-guard.sh` + campo de config `credential_guard.mode`
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `internal/generators/scaffold.go` (novo template de script embutido, seguindo o padrão de
  `signalScript`/`cleanupScript` em `GenerateAttentionScripts`, ~linha 686-733; nova função
  `GenerateCredentialGuardScript(rootDir string) error`)
- `npm/src/generators/scaffold.js` (equivalente Node — localizar função irmã de
  `generateAttentionScripts`)
- `pypi/trackfw/generators/scaffold.py` (equivalente Python)
- `internal/config/config.go` (novo campo `CredentialGuard.Mode` no schema de `trackfw.yaml`,
  default `"warn"`, valores válidos `warn`/`block`)
- Equivalentes de config em `npm/src/config/` e `pypi/trackfw/config/` (localizar pelos nomes
  irmãos de `config.go`)
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
**Status:** ⬜ Pendente
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
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `internal/generators/agentfiles.go` (`InjectCodexHooks`, linha 230-276)
- `npm/src/generators/hooks.js` (linha ~157)
- `pypi/trackfw/generators/hooks.py` (linha ~90)
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
**Status:** ⬜ Pendente
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
- [ ] Nome exato do tool de shell do Gemini CLI documentado em `docs/cli-parity.md` com a fonte
- [ ] `.gemini/settings.json` gerado contém ambos os hooks sem sobrescrever o existente
- [ ] Testes de geração verdes nos 3 stacks
**Comandos de validação:** `go test ./internal/generators/... && npm run test --workspace=npm -- hooks && python -m pytest pypi/tests/ -k hooks`

### ML-2D — GitHub Copilot
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `internal/generators/agentfiles.go` (`InjectCopilotHooks`, linha 363-388)
- `npm/src/generators/hooks.js` (linha ~214)
- `pypi/trackfw/generators/hooks.py` (linha ~210 — **atenção**: este arquivo já diverge do Go/Node em
  estrutura, ver Wave 1 do ADR; usar este ML para também corrigir a divergência de formato
  `{event,run}` vs `{version,hooks:{preToolUse:[...]}}` confirmando qual é o formato real do Copilot
  via `docs.github.com/en/copilot/reference/hooks-reference`)
**Ações:**
- Confirmar o formato real de `.github/hooks/hooks.json` do Copilot (a doc pesquisada usa
  `preToolUse`/`postToolUse` como chaves de nível superior — determinar qual dos dois formatos hoje
  gerados pelos stacks está correto e alinhar os 3 antes de adicionar o novo hook).
- O formato atual do trackfw para Copilot não tem campo de matcher por tool — o filtro para "só
  Bash" precisa acontecer dentro do próprio `trackfw-credential-guard.sh` inspecionando o payload
  recebido via stdin (`tool_name`/`tool.name`, confirmar chave exata do payload do Copilot).
- Adicionar entrada `preToolUse`/`postToolUse` apontando para o script, preservando a existente.
**Critérios de aceite:**
- [ ] Divergência de formato Go/Node vs Python corrigida e documentada em `docs/cli-parity.md`
- [ ] `.github/hooks/*.json` gerado idêntico em estrutura nos 3 stacks
- [ ] Testes de geração verdes nos 3 stacks
**Comandos de validação:** `go test ./internal/generators/... && npm run test --workspace=npm -- hooks && python -m pytest pypi/tests/ -k hooks`

### ML-2E — Cursor
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `internal/generators/agentfiles.go` (`InjectCursorHooks`, linha 391-432)
- `npm/src/generators/hooks.js` (linha ~235)
- `pypi/trackfw/generators/hooks.py` (linha ~237)
**Ações:**
- Migrar (ou adicionar em paralelo) do evento genérico `preToolUse` hoje usado pelo trackfw para o
  evento nativo `beforeShellExecution` (Bash-specific, confirmado por doc oficial pesquisada —
  `docs.cursor.com`/blog GitButler), que já retorna decisão `allow`/`deny`/`ask` — mapear `warn` do
  trackfw para resposta que não bloqueia (permitir + registrar) e `block` para `deny`.
- Preservar a entrada `preToolUse` existente do attention-signal (não migrar essa, só adicionar o
  novo hook em `beforeShellExecution`).
**Critérios de aceite:**
- [ ] `.cursor/hooks.json` gerado contém `beforeShellExecution` novo + `preToolUse`/`postToolUse`
      existentes intactos
- [ ] Resposta do script mapeada corretamente para o protocolo `allow`/`deny`/`ask` do Cursor
- [ ] Testes de geração verdes nos 3 stacks
**Comandos de validação:** `go test ./internal/generators/... && npm run test --workspace=npm -- hooks && python -m pytest pypi/tests/ -k hooks`

### ML-2F — Kiro
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `internal/generators/agentfiles.go` (`InjectKiroHooks`, linha 328-359 — **atenção**: arquivo
  `.kiro/hooks/trackfw-attention.json` é dedicado/overwrite total, comentário explícito no código;
  o novo hook precisa ser adicionado ao mesmo array reescrito, não em arquivo separado, a menos que
  se decida criar `.kiro/hooks/trackfw-credential-guard.json` como segundo arquivo dedicado — avaliar
  qual opção é mais segura e documentar a escolha no commit)
- `npm/src/generators/hooks.js` (linha ~187)
- `pypi/trackfw/generators/hooks.py` (linha ~184)
**Ações:**
- Investigar se o `PreToolUse`/`tool_name` matcher do Kiro de fato intercepta antes da execução de
  um comando Bash (a doc pública pesquisada descreve hooks orientados a `PostFileSave`/eventos de
  IDE — confirmar se `PreToolUse` é um evento realmente disparado por tool-call de shell, não só por
  save de arquivo, antes de prosseguir).
- Se confirmado: adicionar `{event:"PreToolUse", matcher:{tool_name:"Bash|bash|shell"}}` (regex a
  confirmar pelo nome real do tool no Kiro) apontando para o script.
- Se não confirmado: documentar a limitação e mover Kiro para uma wave separada/fora de escopo nesta
  REQ — **não implementar um hook que não intercepta de fato antes da execução**.
**Critérios de aceite:**
- [ ] Resultado da investigação documentado em `docs/cli-parity.md` (confirmado ou limitação
      registrada)
- [ ] Se confirmado: `.kiro/hooks/*.json` gerado com o novo hook, sem quebrar o existente
- [ ] Se não confirmado: ML re-escopado para "documentar limitação", sem código novo, e REQ/roadmap
      atualizados para remover Kiro da wave nativa
**Comandos de validação:** `go test ./internal/generators/... && npm run test --workspace=npm -- hooks && python -m pytest pypi/tests/ -k hooks`

## Wave 3 — Extensão do gate de paridade (1 ML)
> Dependências: Wave 2 completa (precisa dos formatos finais de hooks.json por CLI)

### ML-3A — Estender `check-attention-scripts-parity.sh` para cobrir hooks.json por CLI
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `scripts/check-attention-scripts-parity.sh` (renomear/estender escopo, ou criar
  `scripts/check-credential-guard-hooks-parity.sh` novo, seguindo o mesmo padrão de cenário
  Go-vs-Node-vs-Python já usado no script existente)
- `Makefile` (alvo `parity` — adicionar novo script à cadeia)
- `docs/cli-parity.md` (documentar o novo gate, seção espelhando a já existente para os scripts
  shell, linha ~1599-1631)
**Ações:**
- Para cada CLI da Wave 2, gerar o arquivo de hook via Go/Node/Python num diretório temporário e
  comparar **estruturalmente** (chaves presentes, não byte-a-byte, já que os formatos diferem entre
  CLIs mas devem ser idênticos entre os 3 stacks para o mesmo CLI) — usar `jq` ou parsing equivalente
  no shell.
- Falhar com mensagem clara indicando qual stack diverge e em qual campo.
**Critérios de aceite:**
- [ ] `make quality` (alvo `parity`) roda o novo gate e passa para o estado pós-Wave 2
- [ ] Gate falsifica de propósito (prova negativa, mesmo padrão de `scripts/check-gates-falsify.sh`
      citado em `docs/cli-parity.md`): introduzir divergência manual num stack e confirmar que o
      gate detecta antes de reverter
**Comandos de validação:** `make quality`

## Wave 4 — Teste de sabotagem (1 ML, obrigatório por AC da REQ)
> Dependências: Wave 2 completa (pelo menos ML-2A/Claude Code)

### ML-4A — Teste de sabotagem: materializar JWT sintético e confirmar detecção
**Status:** ⬜ Pendente
**Arquivos afetados:**
- Novo arquivo de teste, ex.: `internal/generators/credential_guard_sabotage_test.go` (Go) +
  equivalentes em `npm/test/` e `pypi/tests/`
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
- [ ] Teste de sabotagem para Claude Code passa e falha propositalmente se o script for removido
      (prova negativa)
- [ ] Cobertura por CLI documentada (quais têm teste de sabotagem, quais não e o motivo)
- [ ] `make test` verde
**Comandos de validação:** `make test`

## Wave 5 — Documentação e encerramento (1 ML)
> Dependências: Waves 1-4 completas

### ML-5A — Atualizar documentação e contexto de trabalho
**Status:** ⬜ Pendente
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
