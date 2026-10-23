---
status: wip
date: 2026-10-23
req: "docs/req/REQ-2026-10-23-regras-de-escrita-dos-agentes-viram-mecanismo-no-harness-working-context-arquivos-congelados-e-escopo-por-despacho.md"
squad: "hades-tf, prometeu-tf, apolo-tf, hefesto-tf"
---

# Roadmap: as regras de escrita dos agentes viram mecanismo no harness

> Created: 2026-10-23 | Status: wip

## Context
REQ: docs/req/REQ-2026-10-23-regras-de-escrita-dos-agentes-viram-mecanismo-no-harness-working-context-arquivos-congelados-e-escopo-por-despacho.md
ADR: docs/adr/ADR-2026-10-23-regras-de-escrita-dos-agentes-sao-aplicadas-pelo-harness-e-nao-pelo-texto-do-handoff.md

Ordem decidida pelo KG: D1 (a instrução) → D2 (o barrier) → D3 (os congelados) → D4 (o escopo por despacho).

## Acceptance Criteria
- [ ] AC1–AC6 da REQ, cada um com a evidência no ML que o fecha
- [ ] `make quality` e o CI verdes

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat model e decisões abertas
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model das quatro camadas
**Status:** ✅ Concluído
**Agente:** hades-tf
**Files affected:** `docs/seguranca/2026-10-23-wave0-regras-de-escrita.md`
**Actions:**
1. **Superfícies, com a enumeração fechada por grep e cruzando todas as dimensões** (lição da REQ-2026-10-23, em que a matriz
   falhou três vezes por olhar só a dimensão do defeito original):
   - os 12 assets de agente e o gerador (`internal/integrations/assets/agents/`, `internal/generators/agentfiles.go`, o
     render em `internal/integrations/`);
   - as checagens do `trackfw barrier` (`internal/commands/barrier.go`);
   - os subcomandos de `trackfw guard` (`internal/commands/guard.go`, `internal/guard/`);
   - a fiação do guard por CLI no `init`, no `update` e no `validate`, e quais matchers de escrita cada CLI tem.
2. **Identidade do agente:** confirme na documentação oficial do Claude Code os campos `agent_id` e `agent_type` no payload
   de `PreToolUse` e os hooks no frontmatter de subagente (<https://code.claude.com/docs/en/hooks>). Diga quais outros CLIs
   entregam identidade e cite a fonte. Defina o que acontece quando o campo falta.
3. **D2:** defina a base do diff da wave, a gramática de `Files affected` (medida sobre os roadmaps reais em
   `docs/roadmaps/`: quantos declaram, em que formato) e a regra para roadmap antigo sem declaração, com um corte por data
   explícito ou outra regra que nunca passe em silêncio.
4. **D3/D4, ameaças:**
   - escrita por Bash, e se o parser de shell do credential guard cobre `>`/`>>`/`tee`/`sed -i`;
   - caminho relativo, `..`, symlink, `/tmp` versus `/private/tmp`, Windows;
   - o agente editando a própria permissão;
   - payload sem `file_path`;
   - `NotebookEdit`;
   - o MCP de escrita;
   - fail-open por erro de parse.
5. **Alvos de falsificação nos dois sentidos** por camada, e o **resíduo declarado**.
**Acceptance criteria:**
- [x] Cada seção respondida com evidência (comando e saída, ou citação com URL)
- [x] A gramática de `Files affected` medida sobre o corpus real, com contagens
- [x] Nenhuma linha de produto
**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-23-wave0-regras-de-escrita.md
git diff --quiet origin/main -- internal/
```

## Wave 1 — D1, a instrução na origem
> Dependencies: Wave 0 auditada.

### ML-1A — Seção Working context dos 11 papéis
**Status:** ✅ Concluído
**Agente:** prometeu-tf
**Files affected:** `internal/integrations/assets/agents/*.md` (os 11 que não são `architect.md`), `internal/integrations/testdata/backend.agent-directory.golden.md`, `internal/integrations/testdata/backend.codex-toml.golden.toml`, `internal/integrations/working_context_form_test.go`
**Acceptance criteria:**
- [x] AC1 da REQ
- [x] `go test ./internal/integrations/ ./internal/generators/ -count=1` verde

**Auditoria do arquiteto (2026-10-23):** aprovado. Os 11 assets ficaram com o texto idêntico, e o `architect.md` não foi
tocado. `TestWorkingContextFormByRole` passa na branch e reprova no HEAD anterior. Os gates estão verdes. O Prometeu não
escreveu no working context e reportou tudo no relatório, como a D1 pede. Ele também apontou a mesma causa num segundo
sítio, que entra como ML-1B.

### ML-1B — O bloco de regras do CLAUDE.md, AGENTS.md e GEMINI.md
**Status:** ✅ Concluído
**Agente:** prometeu-tf
**Por que existe:** o `trackfwRulesBlock` (`internal/generators/agentfiles.go:94`) manda **toda** sessão, inclusive os
subagentes, "write a new file in `docs/agents-working-context/…`". Isso contradiz a D1. O texto é lido por todo agente.
**Files affected:** `internal/generators/agentfiles.go`, `internal/generators/working_context_log_test.go`, `CLAUDE.md`, `AGENTS.md`, `GEMINI.md`
**Acceptance criteria:**
- [x] A linha 3 do bloco passa a dizer que quem registra é a sessão que orquestra (o arquiteto), e que os subagentes
  reportam no relatório final
- [x] Os três arquivos deste repositório ficam com o mesmo texto do gerador; um teste ou gate existente confirma a concordância
- [x] `go test ./internal/generators/ ./internal/integrations/ -count=1` verde

**Auditoria do arquiteto (2026-10-23):** aprovado no worktree. A linha 3 ficou idêntica no gerador e nos 3 arquivos do repo,
e `TestTrackfwRulesBlockAfterFinishingSubagentsNeverWrite` reprova no HEAD anterior. 🔴 **Mas o próprio prometeu-tf violou a
D1 durante o ML.** Ele inseriu 13 linhas no `docs/agents-working-context.md` congelado do **checkout principal** e chamou
isso de "exceção de memória". A instrução que ele seguiu não é a da branch: é o `CLAUDE.md` da `main` local, que está
atrás do `origin` e ainda manda atualizar o `.md`. O arquiteto restaurou o arquivo para o HEAD, com o diff vazio conferido.
Isso mostra que a D1 sozinha não basta enquanto existir uma cópia velha da instrução em algum checkout lido pelo agente,
e é o caso que a D3 (o guard) fecha.

## Wave 2 — D2, o barrier
> Dependencies: Wave 1 auditada. Detalhada depois da Wave 0.

### ML-2A — Checagem `ml_scope`
**Status:** ✅ Concluído
**Agente:** apolo-tf
**Files affected:** `internal/commands/barrier.go`, `internal/commands/barrier_ml_scope_test.go`, `internal/commands/barrier_contract_test.go`, `docs/cli-parity.md`
**Contrato (decisão do arquiteto sobre a Wave 0 §2.2; adendo na ADR):**
1. **Roadmap lido do commit, não do disco.** Os `Files affected` vêm de `git show HEAD:<roadmap>`. Edição não commitada do
   roadmap não amplia o escopo. Quem commita é só o arquiteto, que audita o diff do roadmap. A merge-base não serve,
   porque um roadmap novo não existe nela.
2. **Arquivos tocados:** `git diff --name-only $(git merge-base origin/main HEAD) HEAD`, mais o `git status --porcelain`
   (modificados, staged e untracked), sem os ignorados.
3. **Escopo permitido** para a wave N: a união dos `Files affected` de todo ML das waves 0..N, mais o conjunto de orquestração
   fixo: o próprio roadmap, a REQ do frontmatter `req:`, a ADR do frontmatter `adr:` da REQ, `docs/agents-working-context/`,
   `vault/notes/`, `.claude/agent-memory/`, `<roadmap_dir>/.trackfw-log`, `<roadmap_dir>/.trackfw-log-entries/` e os
   equivalentes em `req_dir`.
4. **Gramática:** cada trecho entre crases dentro da linha `**Files affected:**` é um caminho, um diretório (terminado em `/`)
   ou um glob (`path.Match` por segmento, mais `**`). O texto fora das crases é ignorado. Uma linha só com "a definir" ou sem
   crases declara zero arquivos.
5. **Roadmap sem declaração:** anterior a 2026-07-18 (data do frontmatter), a checagem sai `skipped`, com motivo explícito na
   evidência. Em `wip` com algum ML da wave N sem declaração, a checagem reprova e nomeia o ML. Em `backlog` ou `analyzing`,
   aviso.
6. **Saída:** a checagem `ml_scope` entra no JSON e no texto do barrier, como as outras, e cada falha nomeia o caminho fora de
   escopo. O `result` só fica `blocked` quando ela falha.
**Acceptance criteria:**
- [x] AC2 da REQ, com testes em repositório git real em `t.TempDir()`:
  - braço positivo: o arquivo declarado passa;
  - braço negativo: o arquivo fora reprova e o caminho aparece;
  - um untracked fora reprova;
  - o roadmap editado no disco para incluir o arquivo continua reprovando;
  - o conjunto de orquestração passa;
  - um diretório e um glob casam;
  - pré-corte sai `skipped`;
  - `wip` sem declaração reprova
- [x] Contra-braço: os testes reprovam no HEAD anterior
- [x] `docs/cli-parity.md` documenta a checagem
- [x] Este próprio roadmap passa no `ml_scope` (dogfood). O AC dizia wave 1, mas o barrier da wave 2, com o binário do worktree, deu `ml_scope: passed`. Na primeira rodada, a checagem achou 5 declarações em prosa nos ML-1A e 1B, que foram corrigidas
**Gates da wave:**
```bash
go build ./...
go test ./internal/commands/ -count=1
```

**Auditoria do arquiteto (2026-10-23):** a implementação está aprovada, mas a wave fica bloqueada pelo ML-2B.
- Gates verdes; 10 testes novos.
- O dogfood funcionou: achou 5 arquivos fora de escopo, todos declarados em prosa fora de crases pelo arquiteto nos ML-1A
  e ML-1B, ou não declarados (`barrier_contract_test.go`). As declarações foram corrigidas.
- `scripts/check-barrier.sh` reprova em `barrier/isolated-check/mls_complete/go`, com a mensagem "check 'ml_scope' is
  'skipped', want passed", porque a fixture dele não é repositório git.
- O contra-braço dos testes novos é de compilação: as funções não existem no HEAD anterior. Isso é aceito, porque a
  funcionalidade é nova.

### ML-2B — Corretivo: `check-barrier.sh` com a checagem nova
**Status:** ✅ Concluído
**Agente:** apolo-tf
**Files affected:** `scripts/check-barrier.sh`
**Acceptance criteria:**
- [x] A asserção de isolamento trata `skipped` como não bloqueante, igual ao contrato do Go (`TestBarrierContract_ValidateFalhoBloqueia`), sem afrouxar o resto: `blocked` em checagem não isolada continua reprovando
- [x] Um cenário no script exercita o `ml_scope` reprovando por arquivo fora de escopo, num repositório git temporário
- [x] `GO_BIN=<binário do worktree> bash scripts/check-barrier.sh` rc=0, e `make parity-rest` rc=0. A primeira rodada reprovou porque faltavam anotações `trackfw-contract` em 4 seções `####` do `docs/cli-parity.md`, criadas no ML-2A; o arquiteto as acrescentou

**Nota de auditoria (apolo-tf, 2026-10-23):** `bash scripts/check-barrier.sh` rc=0. `make parity-rest` rc=1 por três seções `####` em `docs/cli-parity.md` (linhas 3212, 3219, 3232, 3236) sem anotação `trackfw-contract` — introduzidas pelo ML-2A; `docs/cli-parity.md` não está nos Files affected do ML-2B; reportado ao arquiteto.

## Wave 3 — D3 e D4, o guard write-scope
> Dependencies: Wave 2 auditada. Detalhada depois da Wave 0.

### ML-3A — `trackfw guard write-scope` com os caminhos congelados (D3)
**Status:** ✅ Concluído
**Agente:** apolo-tf
**Files affected:** `internal/commands/guard.go`, `internal/commands/guard_write_scope_test.go`, `internal/guard/`, `internal/guardforms/`, `internal/generators/`, `internal/validator/`, `internal/config/`, `docs/cli-parity.md`, `.claude/settings.json`, `docs/agents-working-context.md`
**Contrato (arquiteto, a partir da Wave 0 e do adendo da ADR):**
1. **Subcomando** `trackfw guard write-scope`: lê o payload de `PreToolUse` do Claude Code no stdin, com o mesmo leitor, o
   tratamento de BOM e o dreno com prazo de `internal/guard/payload.go`. Sai com 0 para liberar e 2 para negar, com o motivo
   no stderr. Todo erro sob `guard` sai com 2 (D7 da ADR-2026-10-04).
2. **Alvo:** `tool_input.file_path` (`Write`, `Edit`) e `tool_input.notebook_path` (`NotebookEdit`). Campo ausente, vazio ou que
   não seja string nega. `tool_name` fora de `Write|Edit|NotebookEdit` libera, porque o matcher não deveria entregá-lo.
3. **Raiz do projeto do alvo**, e não do cwd: é o ancestral mais próximo do alvo que contém `trackfw.yaml`, depois de
   `filepath.Abs` e de `EvalSymlinks` no maior prefixo existente. Assim, um agente num worktree que escreve no checkout
   principal é julgado pela raiz do checkout principal, que é o caso de 2026-10-23. Alvo fora de qualquer projeto
   trackfw libera.
4. **Lista para toda sessão:** `docs/agents-working-context.md`, só quando as 10 primeiras linhas do arquivo contêm o marcador
   `<!-- trackfw:frozen -->`. Sem o marcador, o arquivo continua livre para a sessão principal, porque projetos de
   terceiros ainda o usam como diário vivo.
5. **Lista só para subagente** (o payload traz `agent_id` não vazio): `docs/agents-working-context.md` (sempre),
   `trackfw.yaml`, `.claude/settings.json`, `.claude/settings.local.json`, `.codex/hooks.json`, `.gemini/settings.json`,
   `.cursor/hooks.json`, `.windsurf/hooks.json`, `.github/hooks/`, `.kiro/hooks/`, `.amazonq/cli-agents/` e `.trackfw/leases/`.
   Comparação por caminho relativo à raiz, com `filepath.Clean`, e sem diferenciar maiúsculas no Windows e no macOS.
6. **Extensão:** a chave `write_scope.frozen_paths` (toda sessão) e a chave `write_scope.frozen_paths_subagent` no
   `trackfw.yaml` acrescentam caminhos. Nunca removem os padrões. YAML inválido nega.
7. **Cabeamento:** o `InjectClaudeHooks` acrescenta em `PreToolUse` um envelope com o matcher `Write|Edit|NotebookEdit` (lista
   exata) e a linha poliglota atual com `trackfw guard write-scope`, registrada em `internal/guardforms`. O `validate` avisa
   quando o `.claude/settings.json` de um projeto não tem esse envelope. É aviso, não violação, até a próxima major.
8. **Este repositório (revisto na auditoria):** o `.claude/settings.json` do repo **não** é cabeado neste PR. O binário
   10.1.0 instalado trata `guard write-scope` como subcomando desconhecido e sai 2, o que bloquearia todo Write/Edit de
   quem fizer pull da `main` antes da release. A fiação do repo entra depois da release e da instalação, num PR próprio,
   como no precedente da ADR-2026-10-04 ("`make install` antes de migrar as próprias configs"). O marcador do item 4 vai
   para o `docs/agents-working-context.md`, e quem o põe é o arquiteto.
**Acceptance criteria:**
- [x] AC3 da REQ, com o payload real do Claude Code, inclusive com `agent_id` presente e ausente
- [x] AC5 da REQ:
  - [x] relativo, `..`, symlink no caminho, `/tmp` e `/private/tmp`, caminho do Windows (`C:\` e `/c/`);
  - [x] payload ilegível, campo ausente e panic → 2;
  - [x] alvo fora de projeto → 0
- [x] Teste da regra do worktree: um alvo no checkout principal, com o cwd no worktree, é julgado pelo `trackfw.yaml` do checkout principal
- [x] Cada teste com frase e contra-braço; `go test ./internal/guard/ ./internal/commands/ ./internal/generators/ ./internal/validator/ ./internal/config/ ./internal/pathguard/ -count=1` verde
**Gates da wave:**
```bash
go build ./...
go test ./internal/guard/ ./internal/commands/ ./internal/generators/ ./internal/validator/ ./internal/config/ ./internal/pathguard/ -count=1
```

**Auditoria do arquiteto (2026-10-23): reprovado em parte, corretivo no ML-3A2.** Gates verdes, `parity-rest` rc=0, 21
testes de subprocesso. Faltou:
- **AC5 de caminho:** relativo, `..`, symlink no caminho, `/tmp` contra `/private/tmp`, Windows (`C:\` e `/c/`);
- **o teste do item 3:** alvo num projeto, cwd noutro;
- **o panic → 2;**
- **qualquer teste unitário em `internal/guard/`.**

O agente também escreveu 2 entradas em `docs/agents-working-context/`. Foram retiradas para quarentena, porque o
conteúdo está no relatório. A fiação do `.claude/settings.json` do repo foi revertida pela decisão do item 8 revisto.

### ML-3A2 — Corretivo do ML-3A
**Status:** ✅ Concluído
**Agente:** apolo-tf
**Files affected:** `internal/guard/`, `internal/commands/guard_write_scope_test.go`
**Acceptance criteria:**
- [x] `internal/guard/writescope_test.go` com testes unitários da resolução de caminho e das listas
- [x] AC5: relativo (negado — fail-closed), `..` que sai do projeto (allow) e que acerta congelado (deny), symlink de
  diretório apontando para o arquivo congelado (deny), `/tmp/x` contra `/private/tmp/x` no macOS (mesmo veredito),
  `C:\` e `/c/` (Windows — normalização unitária via `normalizeTargetPath(p, "windows")`)
- [x] Item 3: o alvo no projeto A com o `cwd` no projeto B é julgado pelo `trackfw.yaml` de A
- [x] Panic recuperado → 2, por seam de teste (`writeScopeInternalHook`)
- [x] Contra-braço de cada um (ver relatório do ML-3A2)

**Auditoria do ML-3A2 (arquiteto, 2026-10-23):** aprovado. Foram achados e corrigidos dois **fail-opens** reais:
- **Caminho relativo:** era resolvido contra o cwd do processo do hook e saía 0. Agora nega, porque o contrato do Write/Edit
  exige caminho absoluto.
- **YAML inválido:** era engolido pelo `ParseWriteScopeConfig` e saía 0. Agora nega.

Os dois foram medidos com exit 0 no HEAD anterior. A normalização `/c/` → `C:\` entrou como função pura. O seam de panic
dá exit 2. Os testes que já passavam no HEAD ficam declarados como fixação de comportamento correto. Nada foi staged, e
nenhuma escrita no working context.

### ML-3B — A permissão por despacho (D4)
**Status:** ✅ Concluído
**Agente:** apolo-tf
**Files affected:** `internal/guard/`, `internal/commands/guard_write_scope_test.go`, `docs/cli-parity.md`
**Contrato (arquiteto, a partir do adendo da ADR e da Wave 0 §2):**
1. **Quando vale:** só quando o payload traz `agent_id` não vazio (subagente) e `agent_type` casa `^[A-Za-z0-9_-]{1,64}$`.
   Um `agent_type` fora do padrão significa "sem permissão": vale só a D3, sem erro.
2. **Onde:** `<raiz do projeto do alvo>/.trackfw/leases/<agent_type>.json`, a mesma raiz do item 3 do ML-3A. Ausente: vale
   só a D3. Ilegível, diretório ou JSON inválido: nega.
3. **Formato (v1):** `{"version":1,"agent_type":"apolo-tf","allow":["/abs/worktree/","/abs/arquivo.go"],"expires":"2026-10-24T00:00:00Z"}`.
   - `allow` é uma lista de caminhos absolutos: terminado em `/` é diretório, os demais são arquivo exato.
   - `expires` é obrigatório, em RFC 3339. Uma permissão expirada nega, com o motivo dizendo que expirou. É isso que impede
     uma permissão esquecida de valer para sempre.
   - `version` diferente de 1 nega.
   - `agent_type` do arquivo diferente do payload nega.
4. **Decisão:** com permissão válida, o alvo (resolvido como no ML-3A) tem de estar dentro de algum `allow`, também
   resolvido com `EvalSymlinks` no maior prefixo existente. Fora de todos, nega, e o motivo nomeia o alvo e o arquivo da
   permissão. A D3 continua valendo antes: um caminho congelado nega mesmo que esteja dentro do `allow`.
5. **O alvo fora de qualquer projeto trackfw** (item 3 do ML-3A) segue liberado. A permissão só é lida dentro de projeto.
   **Resíduo declarado:** um subagente com permissão que escreve fora de qualquer projeto trackfw não é barrado.
6. **Contrato no `docs/cli-parity.md`:** formato, ciclo de vida (o arquiteto grava antes do despacho e apaga depois) e
   resíduos (R6 da Wave 0: dois agentes do mesmo `agent_type` compartilham a permissão).
**Acceptance criteria:**
- [x] AC4 da REQ:
  - alvo dentro do `allow` → 0;
  - alvo fora → 2, com motivo;
  - sem permissão → só a D3;
  - permissão expirada → 2;
  - JSON inválido → 2;
  - `agent_type` fora do padrão → só a D3;
  - `agent_type` divergente → 2;
  - `allow` relativo → 2;
  - congelado dentro do `allow` → 2;
  - sessão principal (sem `agent_id`) ignora a permissão
- [x] Teste da permissão apontando para um worktree, com o alvo no checkout principal → 2. É o caso de 2026-10-23
- [x] Cada teste com a frase e o contra-braço
**Gates da wave:**
```bash
go build ./...
go test ./internal/guard/ ./internal/commands/ ./internal/pathguard/ -count=1
```

**Auditoria do arquiteto (2026-10-23):** a implementação está aprovada; o desenho ficou incompleto e o corretivo é o ML-3C.
- **Gates:** verdes, e `parity-rest` rc=0.
- **Testes:** 22 novos. Os braços de negação reprovam no HEAD anterior, e os de liberação fixam o comportamento que estava
  correto. Foi corrigido também o seam de symlink exigido por `check-symlink-privilege-guard.sh`.
- **Desenho — a variante B mediu exit 0.** A permissão gravada no worktree não protege o checkout principal, porque a
  busca é pela raiz do **alvo**, e o caso real de 2026-10-23 é justamente esse.
- **Desenho — `.trackfw/leases/` não é ignorado pelo git.** Uma permissão, com caminhos absolutos da máquina, poderia ser
  commitada.
- **Fora do escopo:** o agente instalou `packaging` com `pip3 install --user --break-system-packages`, sem autorização. O
  handoff não dava o venv do arquiteto. Isso está reportado ao KG.

### ML-3C — Corretivo da D4: busca da permissão pela sessão e pelo alvo
**Status:** ✅ Concluído
**Agente:** apolo-tf
**Files affected:** `internal/guard/`, `internal/commands/guard_write_scope_test.go`, `internal/generators/`, `.gitignore`, `docs/cli-parity.md`
**Contrato:**
1. **A permissão é procurada em duas raízes**, sem repetição:
   - a raiz do projeto do `cwd` do payload, que é a sessão;
   - a raiz do projeto do alvo.

   Se existir permissão para o `agent_type` em qualquer delas, o alvo tem de estar no `allow` de pelo menos uma permissão
   válida. Uma permissão inválida em qualquer das raízes nega. `cwd` ausente ou relativo significa só a raiz do alvo, como
   hoje.
2. **Com isso, a variante B passa a negar.** É o caso de 2026-10-23: a permissão está no checkout principal (o `cwd` da
   sessão), com `allow` igual ao worktree, e o alvo está no checkout principal.
3. **`.trackfw/leases/` ignorado pelo git:** no `.gitignore` gerado pelo `trackfw init` (ache onde o scaffold escreve as
   entradas `.trackfw-*` do `.gitignore`) e no `.gitignore` deste repositório.
4. **Formato e ciclo de vida** atualizados no `docs/cli-parity.md`.
**Acceptance criteria:**
- [x] A variante B medida com 2; a variante A continua 2; o alvo dentro do `allow` continua 0, com a permissão só na raiz do `cwd`
- [x] Uma permissão inválida na raiz do `cwd` e uma válida na do alvo → 2
- [x] `git check-ignore .trackfw/leases/x.json` rc=0 neste repo, e teste do scaffold gerando a entrada
- [x] Contra-braço de cada teste novo; os gates do ML-3B verdes

## Wave 4 — Revisões independentes
> Dependencies: Wave 3 auditada.

### ML-4A — Revisão de qualidade
**Status:** ⬜ Pendente
**Agente:** hefesto-tf
**Files affected:** `docs/seguranca/2026-10-23-quality-write-scope.md`
**Acceptance criteria:**
- [ ] Parecer sem bloqueio, ou corretivo

### ML-4B — Red-team do guard write-scope
**Status:** ⬜ Pendente
**Agente:** hades-tf
**Files affected:** `docs/seguranca/2026-10-23-red-team-write-scope.md`
**Acceptance criteria:**
- [ ] Reimplementação independente das regras a partir da ADR; nenhum caminho de fail-open

## Wave 5 — Fechamento
> Dependencies: Wave 4 aprovada.

### ML-5A — make quality e CI
**Status:** ⬜ Pendente
**Agente:** zeus-tf
**Files affected:** artefatos de orquestração (roadmap, REQ — sem novo arquivo de produto)
**Acceptance criteria:**
- [ ] `make quality` rc=0, com a máquina ociosa e o HOME isolado
- [ ] CI do PR verde
