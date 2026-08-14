---
status: backlog
date: 2026-08-14
req: "docs/req/REQ-2026-08-14-bloqueio-tecnico-de-comandos-git-brutos-por-subagente-via-deny-hooks-nos-7-runtimes-suportados.md"
squad: ""
---

# Roadmap: bloqueio tecnico de comandos git brutos por subagente via deny/hooks nos 7 runtimes suportados

> Created: 2026-08-14 | Status: backlog

## Context
<!-- Derived from REQ: REQ-2026-08-14-bloqueio-tecnico-de-comandos-git-brutos-por-subagente-via-deny-hooks-nos-7-runtimes-suportados.md -->
REQ: docs/req/REQ-2026-08-14-bloqueio-tecnico-de-comandos-git-brutos-por-subagente-via-deny-hooks-nos-7-runtimes-suportados.md

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ] Os 7 runtimes (claude, codex, gemini, copilot, windsurf, amazonq, cursor) recebem
      configuração técnica de deny/hook para `git commit`, `git push`, `git checkout -b`
      brutos, gerada nos 3 CLIs (Go/Node/Python) com paridade de contrato.
- [ ] Claude Code, Gemini CLI e Amazon Q Developer preservam o arquiteto (Zeus/equivalente)
      com git irrestrito; os demais 4 runtimes aplicam deny global documentado.
- [ ] `make quality` passa sem novas divergências de paridade.

## Diagnóstico / Contexto
Ver REQ vinculada para o levantamento completo por runtime. Resumo do mecanismo escolhido:
reaproveitar o padrão já maduro de `internal/generators/agentfiles.go` usado pelo
credential-guard (`InjectClaudeHooks`, `InjectCodexHooks`, `InjectGeminiHooks`,
`InjectCopilotHooks`, `InjectCursorHooks`, `InjectWindsurfHooks` + equivalente Amazon Q a
criar) — mesmo padrão de merge idempotente (`mergeClaudeHookArray`/`mergeSimpleCommandArray`,
migração de comando obsoleto via `migrateHookCommand`, dedup contra instalação global via
`globalCredentialGuardInstalledX`) — em vez de inventar um mecanismo novo. Um novo script
`scripts/trackfw-git-branch-guard.sh` (irmão de `scripts/trackfw-credential-guard.sh`) decide
allow/deny/block lendo o comando via stdin/args conforme o contrato de cada runtime.

## Wave 1 — Design do guard script e do contrato por runtime (1 ML, bloqueante)
> Dependências: nenhuma — bloqueia a Wave 2 inteira (o script e o contrato de payload
> precisam existir antes de qualquer runtime ser fiado a ele)

### ML-1A — Script guard compartilhado + tabela de contrato por runtime
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `scripts/trackfw-git-branch-guard.sh` (novo)
- `docs/cli-parity.md` (nova seção "Git branch guard por runtime")
**Ações:**
1. Criar `scripts/trackfw-git-branch-guard.sh`, POSIX sh, mesmo estilo defensivo de
   `scripts/trackfw-credential-guard.sh` (fail-closed configurável, sem dependências
   externas além de `git`/`grep`). Lógica: recebe o comando shell completo (via stdin JSON
   nos runtimes que passam JSON — Claude/Gemini/Windsurf/Amazon Q — ou via argv nos que
   passam string crua); casa contra os padrões `^git (commit|push|checkout -b)\b`
   (cobrindo variantes com flags antes, ex: `git -C . commit`); se casar E o comando
   `trackfw` não estiver na mesma linha (heurística: o wrapper nunca invoca git raw
   diretamente pelo agente, ele mesmo faz `exec.Command("git", ...)` internamente, então
   um `git commit` vindo do agente é sempre um bypass), devolve decisão de bloqueio no
   formato esperado por aquele runtime (`{"decision":"block","reason":"..."}` para
   Claude/Gemini estilo JSON-stdout; exit code 2 para Codex/Windsurf estilo exit-code;
   `permission: "deny"` JSON para Cursor). Mensagem de bloqueio deve orientar
   explicitamente: "use `trackfw branch new`/`trackfw ship` — ver CLAUDE.md §1".
2. Escrever em `docs/cli-parity.md` uma tabela "Git branch guard por runtime" com 3
   colunas: runtime | mecanismo usado (deny estático vs hook) | isolamento do
   arquiteto (nativo / via hook / não suportado — deny global) — transcrita da tabela já
   levantada na REQ.
**Critérios de aceite:**
- [ ] `shellcheck scripts/trackfw-git-branch-guard.sh` sem erros
- [ ] script testado manualmente com os 3 formatos de payload (stdin JSON, argv, exit-code)
      simulados via casos de teste em `scripts/trackfw-git-branch-guard_test.sh` (novo,
      mesmo padrão de `*_test.sh` já usado para credential-guard, se existir; senão criar)
**Comandos de validação:** `shellcheck scripts/trackfw-git-branch-guard.sh`

## Wave 2 — Implementação por CLI (3 MLs em paralelo — arquivos distintos por stack)
> Dependências: Wave 1 completa

### ML-2A — Go (`internal/generators/agentfiles.go`)
**Status:** ⬜ Pendente
**Arquivos afetados:** `internal/generators/agentfiles.go`, `internal/generators/agentfiles_test.go`
**Ações:**
1. Em `InjectClaudeHooks`: adicionar entrada `PreToolUse`/matcher `Bash` apontando para
   `$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh`, seguindo exatamente o padrão
   de merge idempotente + migração já usado nas linhas 213-267 para o credential-guard
   (reindexar constantes/mensagens, não duplicar lógica).
2. Em `InjectCodexHooks`: emitir `prefix_rule(pattern=["git","commit"], decision="forbidden")`
   e equivalentes para `push`/`checkout -b` no arquivo de Rules do Codex (ou, se o hook
   `PreToolUse` experimental já estiver estável o suficiente na versão mínima suportada,
   usar o mesmo script guard — decidir e documentar a escolha em `docs/cli-parity.md`).
3. Em `InjectGeminiHooks`: registrar hook `PreToolUse`/`BeforeTool` apontando para o guard
   script (exit code 2 bloqueia), e — como Gemini já suporta subagentes nativos — gerar
   também a config de toolset restrito para os agentes especialistas (`~/.gemini/agents`
   ou `.gemini/agents` do projeto) deixando o arquiteto fora dessa restrição.
4. Em `InjectCopilotHooks`: emitir `--deny-tool='shell(git commit)'` +
   `--deny-tool='shell(git push)'` + `--deny-tool='shell(git checkout:-b)'` em
   `permissions-config.json`/`settings.json` do Copilot CLI — deny global, sem exceção
   por agente (não suportado neste runtime, ver REQ).
5. Em `InjectCursorHooks`: registrar hook `beforeShellExecution` apontando para o guard
   script (payload JSON via stdin, retorno `permission: "deny"`), e adicionar deny estático
   `Shell(git:commit)`/`Shell(git:push)` em `.cursor/rules` como camada extra (defesa em
   profundidade, já que a doc do Cursor avisa que allowlist sozinha não é boundary de
   segurança).
6. Em `InjectWindsurfHooks`: registrar hook `pre_run_command` apontando para o guard
   script (exit code 2 bloqueia) + entrada na deny list `windsurf.cascadeCommandsAllowList`.
7. Criar `InjectAmazonQHooks` (não existe hoje — só há geração de
   `.amazonq/developer/guidelines.md` textual): registrar hook `preToolUse` com
   `matcher: "execute_bash"` apontando para o guard script, `deniedCommands` regex
   (`^git (commit|push|checkout -b)`) em `toolsSettings.execute_bash`, e — como Amazon Q
   também suporta custom agents nativos — `tools`/`allowedTools` restrito para os
   especialistas, arquiteto fora da restrição.
8. Atualizar `InjectRulesForTool`/`InjectRulesDetected` (linhas ~138-181) para despachar
   também para `InjectAmazonQHooks` quando o tool detectado for `amazonq`.
**Critérios de aceite:**
- [ ] `go build ./...` sem erros
- [ ] `go test ./internal/generators/...` verde, incluindo casos novos para os 7 runtimes
- [ ] `go vet ./...` sem warnings
**Comandos de validação:** `go build ./... && go test ./internal/generators/... && go vet ./...`

### ML-2B — Node.js (`npm/src/integrations/`)
**Status:** ⬜ Pendente
**Arquivos afetados:** `npm/src/integrations/assets/agents/` + equivalente de
`agentfiles.go` no Node (localizar módulo irmão de hooks/credential-guard em `npm/src/`,
mesmo diretório que gera `.claude/settings.json`/`.cursor/rules`/etc. hoje)
**Ações:** replicar 1:1 a lógica descrita nos passos 1-8 do ML-2A, reescrita em JS puro,
reaproveitando as funções `merge*`/`migrate*` já existentes no módulo Node equivalente a
`agentfiles.go` (buscar por nome de função espelhado, ex. `injectClaudeHooks`).
**Critérios de aceite:**
- [ ] `npm test --workspace=trackfw` (ou script de teste do workspace Node) verde
- [ ] contrato de saída (JSON/config gerados) idêntico byte-a-byte ao produzido pelo Go
      para o mesmo input, exceto onde a REQ documentar divergência intencional
**Comandos de validação:** `npm test --workspace=npm` (ajustar para o nome real do
workspace conforme `package.json`)

### ML-2C — Python (`pypi/trackfw/integrations/`)
**Status:** ⬜ Pendente
**Arquivos afetados:** `pypi/trackfw/integrations/assets/agents/` + módulo equivalente de
hooks em `pypi/trackfw/`
**Ações:** replicar 1:1 a lógica descrita nos passos 1-8 do ML-2A, reescrita em Python
puro, reaproveitando as funções equivalentes de merge/migração já existentes no módulo
Python de hooks (regra de paridade: Python é reimplementação nativa, não wrapper do Go).
**Critérios de aceite:**
- [ ] `pytest pypi/trackfw` verde
- [ ] contrato de saída idêntico ao Go/Node, mesma ressalva do ML-2B
**Comandos de validação:** `python -m pytest pypi/trackfw`

## Wave 3 — Validação cruzada e auditoria de conformidade (1 ML)
> Dependências: Wave 2 completa (os 3 MLs)

### ML-3A — Paridade, gate de contrato e teste manual end-to-end
**Status:** ⬜ Pendente
**Arquivos afetados:** nenhum novo — só execução de gates existentes + teste manual
**Ações:**
1. Rodar `make quality` na raiz — cobre os contratos de paridade Go/Node/Python.
2. Teste manual em Claude Code (ambiente desta sessão): criar um roadmap de teste
   descartável, tentar `git commit`/`git push`/`git checkout -b` bruto como um subagente
   especialista (ex: via `Agent` tool com `subagent_type: apolo-tf`) e confirmar bloqueio
   com a mensagem do guard script; confirmar que `trackfw branch new`/`trackfw ship`
   continuam funcionando normalmente para o mesmo agente; confirmar que o Zeus
   (`zeus-tf`) continua com git irrestrito.
3. Descartar/remover qualquer roadmap de teste criado no passo 2 antes de finalizar.
**Critérios de aceite:**
- [ ] `make quality` verde, sem novas divergências
- [ ] bloqueio confirmado para especialista, git liberado para Zeus, wrapper funcional
- [ ] nenhum artefato de teste residual commitado
**Comandos de validação:** `make quality`

## Wave 4 — Documentação final (1 ML)
> Dependências: Wave 3 completa

### ML-4A — Fechar `docs/cli-parity.md` com o estado real implementado
**Status:** ⬜ Pendente
**Arquivos afetados:** `docs/cli-parity.md`
**Ações:** atualizar a tabela criada no ML-1A com o estado final confirmado (não o
planejado) após a Wave 2/3 — incluindo qualquer divergência descoberta durante a
implementação (ex: hook do Codex ter permanecido experimental e a decisão final ter sido
usar Rules em vez de hook).
**Critérios de aceite:**
- [ ] tabela reflete o comportamento real, não a intenção original
**Comandos de validação:** revisão manual (doc-only, sem gate automatizado)
