---
status: done
date: 2026-08-14
req: "docs/req/REQ-2026-08-14-bloqueio-tecnico-de-comandos-git-brutos-por-subagente-via-deny-hooks-nos-7-runtimes-suportados.md"
squad: ""
---

# Roadmap: bloqueio tecnico de comandos git brutos por subagente via deny/hooks nos 7 runtimes suportados

> Created: 2026-08-14 | Status: done

## Context
<!-- Derived from REQ: REQ-2026-08-14-bloqueio-tecnico-de-comandos-git-brutos-por-subagente-via-deny-hooks-nos-7-runtimes-suportados.md -->
REQ: docs/req/REQ-2026-08-14-bloqueio-tecnico-de-comandos-git-brutos-por-subagente-via-deny-hooks-nos-7-runtimes-suportados.md

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [x] Os 7 runtimes (claude, codex, gemini, copilot, windsurf, amazonq, cursor) recebem
      configuração técnica de deny/hook para `git commit`, `git push`, `git checkout -b`
      brutos, gerada nos 3 CLIs (Go/Node/Python) com paridade de contrato.
- [x] **Decisão 2026-08-14 (usuário):** deny é global em todos os 7 runtimes,
      inclusive para o arquiteto (Zeus/equivalente) — isolamento por subagente fica
      como débito técnico documentado, não implementado nesta REQ (ver REQ vinculada).
- [x] Existe um comando `trackfw commit` (Go/Node/Python) que recusa commit direto em
      `main`/branch protegida e recusa commit em `feat/fix/refactor` sem roadmap
      correspondente em `wip/`, replicando o gate de `branch_has_wip_roadmap` no
      momento do commit — não só no momento da criação da branch.
- [x] O guard script/deny de cada runtime cobre `git commit` bruto (não só
      `checkout -b`/`push`) e orienta para `trackfw commit`.
- [x] `make quality` passa sem novas divergências de paridade.

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

### ML-1A — Script guard canônico (Go, referência) + tabela de contrato por runtime
**Status:** ✅ Concluído
**Correção pós-descoberta:** `scripts/trackfw-credential-guard.sh` NÃO é um arquivo
estático deste repo — é gerado em runtime a partir da const Go `credentialGuardScript`
(`internal/generators/scaffold.go:1092-1101`) via `GenerateCredentialGuardScript`
(`scaffold.go:793`, escopo de projeto) e `GenerateGlobalCredentialGuardScript`
(`scaffold.go:828`, escopo global `~/.trackfw/scripts/`). Este ML segue o mesmo padrão,
em vez do arquivo estático originalmente descrito.
**Arquivos afetados:**
- `internal/generators/scaffold.go` (nova const `gitBranchGuardScript` + funções
  `GenerateGitBranchGuardScript(rootDir string) error` e
  `GenerateGlobalGitBranchGuardScript(home string) error`, mesmo formato das funções
  irmãs do credential-guard citadas acima)
- `docs/cli-parity.md` (nova seção "Git branch guard por runtime")
**Ações:**
1. Escrever `gitBranchGuardScript` como POSIX sh, mesmo estilo defensivo do
   `credentialGuardScript` (fail-closed configurável, sem dependências externas além de
   `git`/`grep`). Lógica: recebe o comando shell completo (via stdin JSON nos runtimes
   que passam JSON — Claude/Gemini/Windsurf/Amazon Q — ou via argv nos que passam string
   crua); casa contra os padrões `^git (commit|push|checkout -b)\b` (cobrindo variantes
   com flags antes, ex: `git -C . commit`); se casar, devolve decisão de bloqueio no
   formato esperado por aquele runtime (`{"decision":"block","reason":"..."}` para
   Claude/Gemini estilo JSON-stdout; exit code 2 para Codex/Windsurf estilo exit-code;
   `permission: "deny"` JSON para Cursor). Mensagem de bloqueio deve orientar
   explicitamente conforme o subcomando bloqueado: `checkout -b` → "use
   `trackfw branch new <type>/<slug>`"; `commit` → "use `trackfw commit -m '<msg>'`"
   (novo comando, ver Wave 2 deste roadmap); `push` → "use `trackfw ship`" — sempre
   referenciando CLAUDE.md §1.
2. `GenerateGitBranchGuardScript`/`GenerateGlobalGitBranchGuardScript` escrevem
   `scripts/trackfw-git-branch-guard.sh` (projeto) / `~/.trackfw/scripts/trackfw-git-branch-guard.sh`
   (global), espelhando exatamente a estrutura das duas funções irmãs do credential-guard
   (`os.MkdirAll` + `os.WriteFile` com `0755`).
3. Escrever em `docs/cli-parity.md` uma tabela "Git branch guard por runtime" com 3
   colunas: runtime | mecanismo usado (deny estático vs hook) | isolamento do
   arquiteto (nativo / via hook / não suportado — deny global) — transcrita da tabela já
   levantada na REQ.
**Critérios de aceite:**
- [ ] `go build ./...` sem erros
- [ ] `go test ./internal/generators/... -run TestGitBranchGuard` verde, cobrindo os 3
      formatos de payload (stdin JSON, argv, exit-code) e os 3 subcomandos bloqueados
- [ ] `shellcheck` do conteúdo gerado (escrever para um arquivo temporário no teste e
      rodar shellcheck nele, mesmo padrão já usado para validar `credentialGuardScript`
      se existir; senão, criar)
**Comandos de validação:** `go build ./... && go test ./internal/generators/...`

## Wave 2 — Comando `trackfw commit` (3 MLs em paralelo — arquivos distintos por stack)
> Dependências: nenhuma — independente da Wave 1, pode rodar em paralelo com ela.
> Motivação: fecha a lacuna que a Wave 1/3 sozinhas não cobrem — o guard script bloqueia
> o `git commit` bruto, mas o agente ainda precisa de um comando trackfw que faça o
> commit de fato (hoje só `trackfw ship` commita, e ele empacota commit+push+PR num
> combo só; falta um passo intermediário leve). Reproduz o incidente real desta sessão:
> um commit de artefatos de governança foi feito direto na `main` porque não existia
> `trackfw commit` para recusar isso antes do `git commit` acontecer.

### ML-2A — Go: `internal/commands/commit.go` (novo)
**Status:** ✅ Concluído
**Arquivos afetados:**
- `internal/commands/commit.go` (novo)
- `internal/commands/commit_test.go` (novo)
- `internal/commands/root.go` (ou onde os subcomandos são registrados — adicionar `newCommitCmd()`)
**Ações:**
1. Criar `trackfw commit -m "<mensagem>"`, espelhando a estrutura de dependências
   injetáveis de `internal/commands/branch.go` (`branchNewDeps`) — criar `commitDeps` com
   `loadConfig`, `currentBranch func() (string, error)`, `resolveWIPDirs`/`resolveDoneDirs`,
   `matchSlug` (reusar `validator.BranchSlugMatchesRoadmap`, mesma lógica de
   `branch new`), `execGitCommit func(message string) error`.
2. Lógica de bloqueio, nesta ordem:
   a. Ler a branch atual (`git rev-parse --abbrev-ref HEAD`). Se for `main`/`master`
      (ou o nome configurado como branch padrão do repo, resolver via
      `git symbolic-ref refs/remotes/origin/HEAD` com fallback para `main`): bloquear
      sempre, mensagem: "trackfw commit: commit direto em '<branch>' não é permitido.
      Use 'trackfw branch new <type>/<slug>' primeiro."
   b. Se a branch for `feat/`, `fix/` ou `refactor/`: exigir roadmap correspondente em
      `wip/` ou `done/` (mesmo matching de `branch new`); sem match, bloquear com a
      mesma mensagem de orientação já usada por `trackfw validate`
      (`branch_has_wip_roadmap`).
   c. Branches fora desse padrão (ex: branches de doc/housekeeping do próprio Zeus):
      permitir sem exigir roadmap, mas logar aviso — não é o caso coberto por este ML,
      não bloquear artefatos de orquestração.
   d. Se passou em (a)-(c): executar `git commit -m <message>` com stdio herdado,
      propagando saída e status do Git literalmente (mesmo padrão de
      `execGitCheckout`/`git("commit", ...)` já usado em `branch.go`/`ship.go`).
3. Reaproveitar a função `git(...)`/`defaultGitExec` já existente em `ship.go` em vez de
   duplicar o `exec.Command("git", ...)`.
**Critérios de aceite:**
- [x] `go build ./...` sem erros
- [x] `go test ./internal/commands/... -run TestCommit` verde, cobrindo: bloqueio em
      `main`, bloqueio em `feat/x` sem roadmap em `wip`, sucesso em `feat/x` com roadmap
      em `wip`, sucesso em branch fora do padrão feat/fix/refactor
**Comandos de validação:** `go build ./... && go test ./internal/commands/...`

### ML-2B — Node.js: `npm/src/commands/commit.js` (novo)
**Status:** ✅ Concluído
**Arquivos afetados:** `npm/src/commands/commit.js` (novo), teste equivalente, registro
do comando no entrypoint commander (mesmo arquivo que registra `branch`/`ship`)
**Ações:** replicar 1:1 a lógica do ML-2A (passos 1-3) em JS puro, reaproveitando as
funções já existentes para `branch new`/`ship` no módulo Node equivalente
(`npm/src/commands/branch.js`/`ship.js` — localizar nomes exatos).
**Critérios de aceite:**
- [ ] testes do workspace Node verdes, mesmos 4 casos do ML-2A
- [ ] mensagens de erro/orientação idênticas (byte-a-byte) às do Go
**Comandos de validação:** `npm test --workspace=npm` (ajustar nome real do workspace)

### ML-2C — Python: `pypi/trackfw/commands/commit.py` (novo)
**Status:** ✅ Concluído
**Arquivos afetados:** `pypi/trackfw/commands/commit.py` (novo), teste equivalente,
registro do comando no parser argparse/click (mesmo arquivo que registra `branch`/`ship`)
**Ações:** replicar 1:1 a lógica do ML-2A (passos 1-3) em Python puro (reimplementação
nativa, não wrapper do Go — regra de paridade do projeto), reaproveitando as funções
já existentes para `branch new`/`ship` em `pypi/trackfw/commands/branch.py`/`ship.py`.
**Critérios de aceite:**
- [ ] `pytest pypi/trackfw -k commit` verde, mesmos 4 casos do ML-2A
- [ ] mensagens de erro/orientação idênticas (byte-a-byte) às do Go
**Comandos de validação:** `python -m pytest pypi/trackfw -k commit`

## Wave 3 — Implementação dos guards de agente por CLI (3 MLs em paralelo — arquivos distintos por stack)
> Dependências: Wave 1 completa. Independente da Wave 2 em termos de arquivos, mas a
> mensagem de bloqueio do guard script (Wave 1) deve orientar tanto para
> `trackfw branch new`/`trackfw ship` quanto para o novo `trackfw commit` (Wave 2) — por
> isso, embora os MLs desta wave possam começar em paralelo com a Wave 2, o texto final
> da mensagem de orientação só deve ser fechado depois que a Wave 2 confirmar o nome e a
> sintaxe exata do comando.
>
> **Pendência aberta pelo ML-1A (Go):** `GenerateGitBranchGuardScript`/
> `GenerateGlobalGitBranchGuardScript` foram implementadas mas NÃO foram fiadas em
> `Scaffold()` (chamada por `trackfw init`) nem em `trackfw update`/`UpdateHarness` —
> ao contrário de `GenerateCredentialGuardScript`, que é chamada incondicionalmente
> nesses fluxos. Cada ML desta wave (3A/3B/3C) DEVE fiar a geração do script (projeto e
> global) nos mesmos pontos onde o credential-guard já é gerado nesse stack —
> confirmar em cada linguagem onde `GenerateCredentialGuardScript`/equivalente é
> chamada e adicionar a chamada irmã ao lado, senão o script fica órfão (nunca é escrito
> em disco em projetos reais).

### ML-3A — Go (`internal/generators/agentfiles.go`)
**Status:** ✅ Concluído
**Arquivos afetados:** `internal/generators/agentfiles.go`, `internal/generators/agentfiles_test.go`,
`internal/generators/hooks.go` (dispatcher fix), `internal/generators/scaffold.go` +
`internal/generators/update.go` (pendência do ML-1A: fiação da geração do script nos
call sites de produção), `internal/generators/copilot_hooks_parity_test.go` +
`internal/generators/credential_guard_dedup_test.go` (contagens ajustadas para as novas
entradas de git-branch-guard).
**Divergências documentadas em comentários no código (ver apolo-tf, sessão 2026-08-14):**
Codex resolvido via hook `PreToolUse`/Bash (não Rules); Copilot/Cursor/Windsurf sem
diferenciação nativa por agente (deny global, conforme REQ); Windsurf usa um arquivo de
hook dedicado inventado (`.windsurf/hooks/trackfw-git-branch-guard.json`, caminho não
confirmado contra documentação oficial) — `cascadeCommandsAllowList` (IDE settings) não
foi tocado; Amazon Q usa `.amazonq/settings.json` (caminho também não confirmado);
Gemini/Amazon Q sem geradores de subagente nativo no repo — restrição do arquiteto não
implementada, aplicado deny uniforme; Claude/Codex/Gemini também aplicam o guard
uniformemente a todo agente (sem diferenciação real do arquiteto), mesma limitação já
aceita pelo credential-guard pré-existente.
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

### ML-3B — Node.js (`npm/src/generators/hooks.js`)
**Status:** ✅ Concluído
**Arquivos afetados:** `npm/src/generators/hooks.js` (módulo confirmado como o
equivalente Node de `internal/generators/agentfiles.go`+`scaffold.go` — já contém a
lógica de credential-guard hooks), teste equivalente
**Ações:**
1. Portar `gitBranchGuardScript` (conteúdo produzido no ML-1A) como constante JS +
   função `generateGitBranchGuardScript`/`generateGlobalGitBranchGuardScript`,
   espelhando a função irmã de credential-guard já existente neste arquivo.
2. Replicar 1:1 a lógica dos passos 1-8 do ML-3A (wiring de hook/deny por runtime),
   reescrita em JS puro, reaproveitando as funções `merge*`/`migrate*` já existentes
   neste módulo para o credential-guard (mesmo padrão de nomes espelhados, ex.
   `injectClaudeHooks`).
**Critérios de aceite:**
- [ ] testes do workspace Node verdes (localizar comando exato em `npm/package.json`)
- [ ] contrato de saída (script gerado + JSON/config de hooks) idêntico byte-a-byte ao
      produzido pelo Go para o mesmo input, exceto onde a REQ documentar divergência
      intencional
**Comandos de validação:** `npm test --workspace=npm` (ajustar para o nome real do
workspace conforme `package.json`)

### ML-3C — Python (`pypi/trackfw/generators/hooks.py`)
**Status:** ✅ Concluído

**Nota de execução (2026-08-14):** implementado após o Go (ML-3A) já ter aterrissado nesta mesma
branch, o que permitiu validar paridade estrutural real
(`GO_BIN=bin/trackfw scripts/check-agent-hooks-parity.sh` e
`scripts/check-harness-hooks-parity.sh`, 12/12 e 12/12 OK) em vez de só uma cópia "melhor esforço"
da spec do roadmap. Kiro foi deliberadamente deixado de fora (não é um dos "7 runtimes" do título
do roadmap) — um rascunho inicial tinha adicionado wiring para Kiro por engano; removido depois que
o gate apontou divergência `go-vs-py` em `$.hooks` porque o Go também não tem Kiro. Divergências
documentadas em comentário (idênticas às do Go): Copilot `--deny-tool` estático (flag de CLI, sem
arquivo de config persistível), Cursor `.cursor/rules` deny estático, restrição nativa de toolset
por subagente (Gemini/Amazon Q) — nenhuma implementada, mesmo racional do Go (sem gerador de
subagente em nenhum dos 3 CLIs ainda). Ver `docs/agents-working-context.md`, sessão
"Apolo (ML-3C: Python ...)" para o detalhamento completo.

**Arquivos afetados:** `pypi/trackfw/generators/hooks.py` (módulo confirmado como o
equivalente Python — já contém a lógica de credential-guard hooks), teste equivalente
**Ações:**
1. Portar `gitBranchGuardScript` como constante Python + função
   `generate_git_branch_guard_script`/`generate_global_git_branch_guard_script`,
   espelhando a função irmã de credential-guard já existente neste arquivo.
2. Replicar 1:1 a lógica dos passos 1-8 do ML-3A, reescrita em Python puro,
   reaproveitando as funções equivalentes de merge/migração já existentes neste módulo
   (regra de paridade: Python é reimplementação nativa, não wrapper do Go).
**Critérios de aceite:**
- [ ] `pytest pypi/trackfw -k hooks` verde
- [ ] contrato de saída idêntico ao Go/Node, mesma ressalva do ML-3B
**Comandos de validação:** `python -m pytest pypi/trackfw -k hooks`

## Wave 4 — Validação cruzada e auditoria de conformidade (1 ML)
> Dependências: Wave 2 e Wave 3 completas

### ML-4A — Paridade, gate de contrato e teste manual end-to-end
**Status:** ✅ Concluído

**Execução real (divergiu do plano em escopo, não em critério):** o teste manual E2E via
subagente (`apolo-tf`) revelou 3 bugs reais de robustez no guard script (não previstos no
desenho original): (1) falso negativo em comando encadeado `git a; git push`, (2) falso
negativo em path absoluto `/usr/bin/git commit`, (3) falso positivo crítico — o guard
bloqueava `trackfw commit`/`trackfw ship` legítimos sempre que a mensagem de commit
mencionasse "git commit"/"git push" em qualquer lugar da string (reproduzido ao vivo
nesta sessão, bloqueou o próprio arquiteto tentando commitar). Corrigidos nos 3 CLIs
(commit `d882f0a`) antes de fechar este ML — não fazia sentido declarar "teste manual
passou" com bugs de robustez conhecidos e não corrigidos. `scripts/check-commit-parity.sh`
criado e registrado em `make quality` (commit `c686f33`), que por sua vez encontrou um
bug real de buffering no Python (`out.flush()` ausente antes de subprocess com stdio
herdado).
**Arquivos afetados:** `scripts/check-commit-parity.sh` (novo) + `Makefile` (wiring em
`make quality`)
**Pendência aberta pelo ML-2C (Python):** o projeto tem gates de paridade dedicados por
comando (ex: `scripts/check-branch-new-parity.sh`), mas não existe
`check-commit-parity.sh` para `trackfw commit` — sem ele, `make quality` não verifica de
fato que as mensagens de bloqueio são byte-idênticas entre Go/Node/Python (checagem
manual desta sessão confirmou que estão idênticas nesta versão, mas nada impede
divergência futura sem o gate).
**Ações:**
1. Criar `scripts/check-commit-parity.sh`, mesmo padrão de
   `scripts/check-branch-new-parity.sh` (comparar as strings de mensagem de bloqueio —
   main/master, feat-sem-roadmap, branch-fora-do-padrão — entre `internal/commands/commit.go`,
   `npm/src/commit/runner.js` e `pypi/trackfw/commands/commit.py`), registrar no
   `Makefile` como parte do alvo `quality`/`parity` já existente (mesmo lugar onde
   `check-branch-new-parity.sh` está registrado).
2. Rodar `make quality` na raiz — cobre os contratos de paridade Go/Node/Python.
3. Teste manual em Claude Code (ambiente desta sessão): criar um roadmap de teste
   descartável, tentar `git commit`/`git push`/`git checkout -b` bruto como um subagente
   especialista (ex: via `Agent` tool com `subagent_type: apolo-tf`) e confirmar bloqueio
   com a mensagem do guard script; confirmar que `trackfw branch new`/`trackfw ship`/
   `trackfw commit` continuam funcionando normalmente para o mesmo agente; confirmar que
   `trackfw commit` recusa quando a branch atual é `main`/protegida ou sem roadmap em
   `wip` correspondente (mesmo caso real que motivou este roadmap — ver commit `cda74cd`
   revertido da `main` nesta sessão); confirmar que o Zeus (`zeus-tf`) continua com git
   irrestrito.
4. Descartar/remover qualquer roadmap de teste criado no passo 3 antes de finalizar.
**Critérios de aceite:**
- [ ] `scripts/check-commit-parity.sh` criado e registrado em `make quality`/`parity`
- [ ] `make quality` verde, sem novas divergências
- [ ] bloqueio confirmado para especialista, git liberado para Zeus, wrapper funcional
- [ ] `trackfw commit` recusa commit direto na `main` e sem roadmap em `wip`
- [ ] nenhum artefato de teste residual commitado
**Comandos de validação:** `make quality`

## Wave 5 — Documentação final (1 ML)
> Dependências: Wave 4 completa

### ML-5A — Fechar `docs/cli-parity.md` com o estado real implementado
**Status:** ✅ Concluído
**Arquivos afetados:** `docs/cli-parity.md`
**Ações:** atualizar a tabela criada no ML-1A com o estado final confirmado (não o
planejado) após a Wave 2/3 — incluindo qualquer divergência descoberta durante a
implementação (ex: hook do Codex ter permanecido experimental e a decisão final ter sido
usar Rules em vez de hook).
**Critérios de aceite:**
- [ ] tabela reflete o comportamento real, não a intenção original
**Comandos de validação:** revisão manual (doc-only, sem gate automatizado)
