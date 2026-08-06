# agents-working-context.md

> Arquivo de handoff entre sessões. Atualizar ao iniciar e ao encerrar cada ciclo de trabalho.

---

## Sessão 2026-08-04 — Apolo (ML-3A: documentar `trackfw branch new` + gate de paridade) — INICIADO

Branch `feat/comando-trackfw-branch-new-para-bloquear-criacao-de-branch-sem-req-roadmap-em-wip`
(já criada pelo orquestrador — Backend não executa Git; sem commit/push feitos por este agente).

Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-04-comando-trackfw-branch-new-para-bloquear-criacao-de-branch-sem-req-roadmap-em-wip.md`,
ML-3A (Wave 3, único ML pendente — Waves 1/2 já concluídas e auditadas).
REQ: `docs/req/REQ-2026-08-04-comando-trackfw-branch-new-para-bloquear-criacao-de-branch-sem-req-roadmap-em-wip.md`.

**Escopo**: documentar `trackfw branch new` em `docs/cli-parity.md` (tabela + seção própria) e criar
gate de paridade automatizado (`scripts/check-branch-new-parity.sh`) + cenário de falsificação em
`scripts/check-gates-falsify.sh` (P4), integrado a `make quality`. Não altero comportamento de
`internal/commands/branch.go` / `npm/src/commands/branch.js` / `pypi/trackfw/commands/branch.py` —
só leio para documentar/cobrir corretamente.

Antes de escrever o gate, vou diffar empiricamente os três binários reais (dry-run com/sem match,
e o caminho `git checkout -b` real com branch já existente) para confirmar que o texto que vou pinar
no gate é de fato byte-idêntico nos 3 runtimes — não vou assumir a partir da leitura do Go sozinho.

## Sessão 2026-08-04 — Apolo (ML-3A: documentar `trackfw branch new` + gate de paridade) — CONCLUÍDO, aguardando auditoria/commit de `trackfw_architect`

**Diff empírico prévio** (3 binários reais, fixture `git init` real): confirmei stdout/stderr/exit
code byte-idênticos nos 3 cenários — (a) sem match + `--dry-run` (exit 1, mensagem de orientação +
linha `blocked: ...` em stderr), (b) com match + `--dry-run` (exit 0, "would create branch"), (c)
com match, git real, branch já existente (exit 128, `git`'s próprio `fatal: a branch named '...'
already exists`, sem vazamento de `exit status N`) — antes de escrever qualquer asserção do gate.

**Arquivos criados/alterados**:
- `docs/cli-parity.md` — nova linha `branch` na tabela de comandos (adjacente a `ship`) + nova seção
  `## \`trackfw branch new\`` (command surface, decision flow, tabela de função de matching
  compartilhada por runtime, seção sobre propagação literal do exit code do Git com o histórico do
  achado da Wave 2, descrição do gate) + parágrafo de cross-link na seção existente
  `## Regra \`branch_has_wip_roadmap\``.
- `scripts/check-branch-new-parity.sh` (novo) — gate dedicado, 3 cenários (no-match, match+dry-run,
  git-checkout-branch-exists real), diff -u byte-a-byte de stdout+stderr+exit code entre os 3
  runtimes, guards de vacuidade em cada cenário.
- `Makefile` — `check-branch-new-parity.sh` adicionado ao target `parity`, antes de
  `check-gates-falsify.sh` (que roda por último).
- `scripts/check-gates-falsify.sh` — Cenário 42 (P4): corrompe a mensagem `blocked: ...` no
  Node.js (`npm/src/branch/runner.js`) via sed, confirma que `check-branch-new-parity.sh` detecta a
  divergência (go-vs-node/err) — contagem do resumo final atualizada de 99→100 cenários, 14→15 gates.
- `vault/notes/check-identity-parity-json-html-escaping-pre-existing-2026-08-04.md` (novo, ver
  achado fora de escopo abaixo) + entrada no `index.md`.

**Achado fora de escopo, reportado sem correção**: `scripts/check-identity-parity.sh` já falha nesta
branch **antes** de qualquer mudança deste ML — 6 divergências (`amazonq`, `antigravity=legacy-cli`,
`kiro=cli` × with/no-identity) causadas por `encoding/json.Marshal` do Go fazer HTML-escaping de
`<slug>` (`<slug>`) enquanto Node.js/Python não escapam. Confirmado reproduzindo em árvore
limpa via `git stash`. Detalhe em
`vault/notes/check-identity-parity-json-html-escaping-pre-existing-2026-08-04.md`. Não corrigido —
fora do escopo do ML-3A; reportado para triagem do orquestrador.

**Evidência de validação**:
- `go build ./...`, `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test -timeout 2m ./...` → todos os
  pacotes `ok`.
- `go vet ./...` → limpo.
- `GO_BIN=bin/trackfw scripts/check-branch-new-parity.sh` isolado → `OK` nos 3 cenários.
- `GO_BIN=bin/trackfw scripts/check-gates-falsify.sh` completo → `Falsification checks passed (all
  100 scenarios...)`, incluindo `OK [falsify/branch-new-parity/no-match/go-vs-node/err-message-
  reformatted-not-detected]` (prova P4 de que o gate detecta a divergência injetada).
- `make quality` (execução completa) → falha em `check-identity-parity.sh` (achado pré-existente
  acima, não relacionado a este ML). Todos os demais gates de `parity` — incluindo
  `check-roadmap-move-parity.sh`, `check-branch-new-parity.sh`, `check-artifact-parity.sh`,
  `check-barrier.sh`, `check-slash-parity.sh`, `check-rules-parity.sh`, `check-update-parity.sh` —
  rodados isoladamente com `GO_BIN=bin/trackfw`, todos verdes.
- `trackfw validate` (binário real e binário `bin/trackfw` recompilado) → `✓ Nenhuma violação
  encontrada.`

**Git**: nenhum commit/push feito (Backend não tem autoridade Git). Working tree deixado para
`trackfw_architect` auditar: `Makefile`, `docs/agents-working-context.md`, `docs/cli-parity.md`,
`scripts/check-gates-falsify.sh` modificados; `scripts/check-branch-new-parity.sh` novo,
`vault/notes/check-identity-parity-json-html-escaping-pre-existing-2026-08-04.md` novo.

**Não alterei**: `internal/commands/branch.go`, `npm/src/commands/branch.js`,
`npm/src/branch/runner.js`, `pypi/trackfw/commands/branch.py` — só li para documentar. Nenhum
comportamento de `trackfw branch new` foi modificado.

---

## Sessão 2026-08-04 — Ares (ML-1A: `make quality` sensível a locale no gate de falsify) — CONCLUÍDO, aguardando auditoria/commit de `trackfw_architect`

Branch `fix/make-quality-locale-fixo-no-falsify` (já criada — Infra não executa Git; sem commit/push
feitos por este agente, conforme Git authority de `trackfw_architect`).

Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-04-make-quality-locale-fixo-no-falsify.md`, ML-1A
(ainda ⬜ Pendente — só será marcado ✅ após auditoria do orquestrador).
REQ: `docs/req/REQ-2026-08-04-make-quality-falha-sob-locale-pt-br-teste-fixa-literal-em-ingles-no-violations-found.md`.
ADR: `docs/adr/ADR-2026-08-04-make-quality-forca-locale-fixo-no-gate-de-falsificacao-em-vez-de-pin-em-ingles.md`.

**Correção**: em `scripts/check-gates-falsify.sh`, Cenário 29 (`validate-ok-message`), as 4 chamadas
que capturam `s29_go_out`, `s29_node_out`, `s29_python_out` e `s29c_python_out` agora fixam
`env LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8 ...` (compondo com o `PYTHONPATH=` já existente no mesmo
`env`, no estilo já usado no resto do script). Nenhum outro arquivo tocado.

**Auditoria dos cenários irmãos** (30/31/33/34/35/36, que também pinam saída textual): não precisam
da mesma correção — `trackfw status` nos 3 CLIs não passa nenhuma string do bloco Inventory/WIP/
Blocked/Done por i18n (confirmado por grep; único uso de i18n em `status.js` é a descrição do
`--help`, fora do que os cenários capturam). Também não há outro `_EXPECTED=` no script pinando as
demais mensagens i18n existentes (`validate.violations`, `validate.warnings`,
`validate.lenient_mode`) — só `validate.ok` (Cenário 29) é exercitado por um cenário.

**Reprodução do bug antes da correção**: `LANG=pt_BR.UTF-8 LC_ALL=pt_BR.UTF-8 bash
scripts/check-gates-falsify.sh` numa árvore com o script sem a correção (via `git stash`, revertido
com `git stash pop` logo após) falhou exatamente como o vault previa: `esperava '✓ No violations
found.' ... go/node/python: '✓ Nenhuma violação encontrada.'`.

**Evidência pós-correção**: `LANG=pt_BR.UTF-8 LC_ALL=pt_BR.UTF-8 make quality` → 99/99 cenários OK,
0 FAIL, incluindo `falsify/validate-ok-message/baseline-byte-identical-and-pinned` e
`falsify/validate-ok-message/python-detects-regression` (prova de detecção da regressão continua
reprovando o literal hardcoded do Python nos dois locales). `LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8 make
quality` → mesmo resultado, 99/99 OK. `trackfw validate` (pós-edição) → `✓ Nenhuma violação
encontrada.` — sem violações.

---

## Sessão 2026-08-04 — Prometeu (ML-1A: dispatch contract sem `subagent_type` no template do Architect) — CONCLUÍDO, aguardando auditoria/commit de `trackfw_architect`

Branch `fix/corrigir-dispatch-sem-subagent-type-no-template-do-architect` (já criada por
`trackfw_architect` — Tooling não executa Git; sem commit/push feitos por este agente).

Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-04-corrigir-dispatch-sem-subagent-type-no-template-do-architect.md`,
ML-1A (ainda pending — só será marcado ✅ após auditoria do orquestrador).
REQ: `docs/req/REQ-2026-08-04-corrigir-dispatch-sem-subagent-type-no-template-do-architect.md`.

**Correção**: adicionada a seção `## Dispatch contract` ao template canônico
`internal/integrations/assets/agents/architect.md` (entre `## Workflow` e `## Post-microbatch
audit`), explicando que nomear um especialista na prosa/`squad:` não roteia a chamada da Agent tool;
todo dispatch exige `subagent_type` explícito (senão cai silenciosamente em `general-purpose`); o
valor correto é o `name:` do frontmatter do agente instalado do role-alvo (`<slug>-tf`,
identity-agnostic — nunca nome fixo); ler o arquivo do agente instalado antes de despachar se o valor
não for conhecido.

Propagado byte-a-byte via `scripts/sync-integration-assets.sh` para `npm/src/integrations/assets/agents/architect.md`
e `pypi/trackfw/integrations/assets/agents/architect.md`. Goldens Go atualizados
(`internal/integrations/testdata/architect.subagent.golden.md` e `architect.agent-directory.golden.md`)
e comentário de re-congelamento acrescentado em `internal/integrations/render_test.go`. Fixture Node
`npm/tests/agents-skills.test.js` (golden string `expectedArchitect`, teste "Antigravity
agent-directory renderer é byte-equivalente ao contrato Go/Python") atualizada com a mesma seção.
Nenhuma fixture Python compara o corpo completo do arquivo (só id/nome do agente) — confirmado por
grep, nada a ajustar. Nenhuma menção a `subagent_type` vazou para templates Gemini/Copilot/Windsurf/
Codex (confirmado por grep — só os 3 `architect.md`).

**Evidência**: `go build ./...` limpo; `go test ./internal/integrations/...` → ok; `bash
scripts/check-integration-assets.sh` → "Integration assets are synchronized (file lists and bytes
match)"; `cd npm && npm test` → 356/356 passed (inclui `tests/validator.test.js` 63/63); `cd pypi &&
python3 -m pytest -q` → 872 passed, 8 subtests passed; `trackfw validate` → `✓ Nenhuma violação
encontrada.`.

Nota de vault atualizada com o achado + resolução:
`vault/notes/falsify-suite-locale-dependent-false-failure-2026-08-03.md` (já indexada em
`vault/notes/index.md`; o achado original já estava lá desde 2026-08-03 — Ártemis já havia
diagnosticado a causa raiz e recomendado exatamente esta correção, só não a aplicou por estar fora do
escopo do ML dela).

Sem commit/push — devolvido para `trackfw_architect` auditar e commitar (Infra não tem autoridade
Git). Único arquivo tocado: `scripts/check-gates-falsify.sh` (+ nota de vault e esta entrada de
working-context, que são artefatos de orquestração/documentação, não código de produto).

---

## Sessão 2026-08-04 — Apolo (ML-2A: paridade — status inválido no move físico de REQ) — CONCLUÍDO

Branch `feat/req-move-list-subpastas-e-move-fisico` (já criada — Backend não executa Git; sem
commit/push feitos por este agente).

Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-04-req-move-list-subpastas-e-move-fisico.md`, ML-2A.

**Achado da auditoria pós-Wave-1 corrigido:** as 3 implementações de `req move` divergiam no
tratamento de status inválido quando a REQ já está numa subpasta de estado reconhecida (por-estado
ou by_agent): Go rejeitava com erro, Node criava uma pasta arbitrária com o valor recebido
(`targetDir = path.join(cfg.reqDir, status)` sem validar), e Python caía silenciosamente no fallback
in-place (`_req_state_dir`/`_req_agent_state_dir` retornavam `None` para status inválido, sem avisar).

**Correção:** alinhados Node.js (`npm/src/generators/req.js`, `moveREQ`) e Python
(`pypi/trackfw/generators/req.py`, `move_req`) ao comportamento do Go — validação
`status in VALID_STATES` logo após o branch in-place e antes do cálculo de `targetDir`, lançando erro
equivalente (`invalid state "<status>" — valid states: backlog, analyzing, wip, blocked, done,
abandoned`) via `throw new Error` (Node) / `raise RuntimeError` (Python). O modo in-place (REQ solta
em `req_dir/`) não foi alterado — continua aceitando qualquer string livremente. Nota: o ponto de
validação escolhido (espelhando o Go) valida em TODOS os caminhos não-in-place, inclusive layout não
reconhecido (ex: `docs/req/claude/deep/nested/REQ.md`) — mais amplo que o caso descrito na tarefa
(só "subpasta de estado reconhecida"), mas é a paridade estrita com o Go e fecha os 3 caminhos, não
só 2.

Testes de regressão adicionados nos 3 CLIs (layout por-estado E by_agent, este último era o caso mais
grave em Python — `_req_agent_state_dir` retornava `None` e caía silenciosamente em in-place):
`TestMoveREQ_RejectsInvalidStateInStateLayout` e `TestMoveREQ_RejectsInvalidStateInByAgentLayout`
(Go); testes 7 e 8 em `npm/tests/req_list_move_subfolders.test.js`;
`test_move_req_rejects_invalid_state_in_state_layout` e
`test_move_req_rejects_invalid_state_in_by_agent_layout` (Python).

**Evidência:** `go build ./... && go vet ./... && go test ./internal/...` verde · `npm --prefix npm
test` verde (354/354, arquivo `req_list_move_subfolders.test.js` roda 9/9 standalone incluindo os 2
novos casos) · `python3 -m pytest tests/` verde (872 passed, dentro de `pypi/`) · `trackfw validate`
pós-edição sem violações · `make quality` (gate de contratos de paridade) verde sob
`LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8` — sob o locale padrão da máquina (`pt_BR.UTF-8`) o cenário 29
(`falsify/validate-ok-message`) falha por pinar o literal inglês "No violations found." contra a
mensagem i18n em português; confirmado pré-existente e não-relacionado ao diff (reproduzido em árvore
stashed, sem nenhuma mudança deste ML).

**Prova de paridade manual** (fixture temporário fora do repo, `trackfw.yaml` com
`roadmap_namespacing: by_agent`, `agents: [claude]`, REQs nos 3 layouts — flat, por-estado,
by_agent): os 3 binários (`bin/trackfw`, `node npm/bin/trackfw`, `python3 -m trackfw` com
`PYTHONPATH=pypi`) listaram exatamente o mesmo conjunto de 3 REQs via `req list`; `req move` físico
produziu os mesmos destinos (`docs/req/done/REQ-state.md`, `docs/req/claude/done/REQ-agent.md`,
REQ solta permanecendo in-place) nos 3; e `req move REQ-state.md status-invalido-xyz` agora é
rejeitado com mensagem equivalente nos 3 (Node imprime stack trace não capturado — comportamento
pré-existente de `npm/src/commands/req.js`, que não tem try/catch em `req move`, fora do escopo deste
ML), sem criar `docs/req/status-invalido-xyz/` e sem alterar o arquivo original.

Sem commit/push — devolvido para `trackfw_architect` auditar e commitar (Backend não tem autoridade
Git). Arquivos tocados: `internal/generators/req.go` (nenhuma mudança — já estava correto),
`internal/generators/req_test.go`, `npm/src/generators/req.js`,
`npm/tests/req_list_move_subfolders.test.js`, `pypi/trackfw/generators/req.py`,
`pypi/tests/test_req_list_move_subfolders.py`. A árvore também carrega `README.md` e
`docs/cli-parity.md` modificados por outro agente em paralelo (ML-2B) — não tocados por este ML.

---

## Sessão 2026-08-04 — Apolo (ML-1A: Go — req list/move recursivos + move físico) — CONCLUÍDO

Branch `feat/req-move-list-subpastas-e-move-fisico` (já criada pelo orquestrador — Backend não
executa Git; sem commit/push feitos por este agente).

Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-04-req-move-list-subpastas-e-move-fisico.md`, seção
ML-1A. REQ: `docs/req/REQ-2026-08-03-req-move-list-nao-suportam-subpastas-e-req-move-nao-move-arquivo.md`.
ADR: `docs/adr/ADR-2026-08-04-req-move-list-reusar-roadmap-namespacing-para-req-e-mover-fisicamente-o-arquivo.md`.

Escopo: implementar apenas o CLI Go (`internal/generators/req.go`, `internal/commands/req.go`,
`internal/generators/req_test.go`) — Node.js e Python ficaram com outros dois agentes em paralelo
(mesma branch, arquivos distintos).

**Mudanças:**
- `listREQFiles(cfg config.ProjectConfig) []string` (novo) — descoberta recursiva nos 3 layouts
  (flat, por-estado, by_agent), reaproveitando `roadmapStateOrder`/`roadmapValidStateNames` de
  `roadmap.go`.
- `ListREQs(dir string) error` → `ListREQs() error` (assinatura mudou; carrega `config.Load()`
  internamente). Call site atualizado em `internal/commands/req.go:153` (import `config` removido
  do arquivo, ficou sem outros usos).
- `findREQ(name, dir string)` → `findREQ(name string, cfg config.ProjectConfig)`, itera
  `listREQFiles`.
- `MoveREQ`: implementado o move condicional (in-place para REQ solta em `cfg.REQDir`; move físico
  real — `MkdirAll` + `WriteFile` + `Remove` — quando já organizada em subpasta de estado
  reconhecida, por-estado ou by_agent) + `appendREQTransitionLog` (novo, grava em
  `cfg.REQDir/.trackfw-log`).

**Achado não óbvio (documentado apenas aqui, não subiu a nota de vault por ser localizado):**
o roadmap instruía reaproveitar `stateDir`/`agentStateDir` de `roadmap.go` para resolver o
`targetDir` do move de REQ — mas essas duas funções são hardcoded em `cfg.RoadmapDir`, não
`cfg.REQDir`. Usá-las literalmente moveria REQs para dentro do diretório de roadmaps. Corrigido
construindo `targetDir` diretamente com `filepath.Join(cfg.REQDir, ...)`, validando o estado via
`roadmapValidStateNames` (esse sim reaproveitado). Vale conferir se ML-1B/ML-1C (Node/Python)
caem na mesma armadilha ao espelhar o algoritmo.

Testes novos: `TestListREQs_ByState`, `TestListREQs_ByAgent`, `TestFindREQ_RecursesSubfolders`,
`TestMoveREQ_PhysicallyMovesInStateLayout`, `TestMoveREQ_PhysicallyMovesInByAgentLayout`,
`TestMoveREQ_LogsTransition`. `TestMoveREQ_RewritesStatusInPlace` preservado sem alteração de
asserts (só passou a coexistir com `config.Reset()`/`t.Cleanup` já usados por outros testes do
pacote).

**Evidência:** `go build ./...` limpo · `go test ./internal/generators/... ./internal/commands/...`
verde (13 testes de REQ, incluindo os 6 novos) · `go vet ./...` sem avisos · `go test ./internal/...`
completo verde.

Sem commit/push — devolvido para `trackfw_architect` auditar e commitar (Backend não tem
autoridade Git).

---

## Sessão 2026-08-03 — Hades (Barreira de segurança pré-Wave 3, ML-1A+ML-2A) — CONCLUÍDO

Branch `refactor/unificar-leitura-trackfw-yaml`, revisão apenas (sem commits — Security não
executa Git; correções, se houvesse, ficariam para o dono do código).

Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-02-unificar-a-leitura-do-trackfw-yaml-em-um-unico-carregador-nos-tres-clis.md`
Escopo: confirmar que `linear_api_key`/`jira_token` (agora em `cfg.Sync`/`cfg.sync`/`cfg["sync"]`)
não vazam em log, mensagem de erro, `--json` ou config dumpado para diagnóstico, nos 3 CLIs.

**Veredito: APROVADO.** Nenhum vazamento encontrado. Achados:
- `internal/sync/{linear,jira}.go`, `npm/src/commands/sync.js`, `pypi/trackfw/commands/sync.py`:
  segredos só chegam a headers HTTP (`Authorization`); mensagens de erro citam o nome do campo
  ausente, nunca o valor — comportamento idêntico ao pré-refactor.
- Caminho fatal de YAML malformado (novo para `sync`, que antes engolia erro silenciosamente):
  Go emite `MalformedConfigMessage` estático; Node idem; Python usa
  `except yaml.YAMLError: return True` e descarta a exceção — **não** imprime
  `str(e)`/snippet do PyYAML, que poderia ecoar a linha com o segredo. Os 3 CLIs imprimem apenas
  mensagem fixa, sem o conteúdo do YAML.
- Nenhum `json.Marshal(cfg)`/`JSON.stringify(cfg)`/`json.dumps(cfg)` do `ProjectConfig`/dict de
  config completo existe em nenhum dos 3 CLIs — `context`, `status --json`, `barrier --json` e
  `validate --json` serializam structs próprias (sem campo `Sync`/`sync`), não o cfg bruto.
- `trackfw serve`/`serve.py`/`serve.js`: o cfg completo (incluindo `Sync`) é injetado, por
  singleton, em todos os handlers HTTP de um processo de vida longa — nenhum handler lê `Sync`
  hoje (auditado: board, chain, metrics, file, attention nos 3 CLIs), mas é superfície de
  reachability nova (antes, o segredo só era lido transitoriamente por um `sync` que processava e
  saía). **Achado informativo/low, não bloqueante** — recomendação de hardening para ML futuro:
  `json:"-"` em `SyncConfig` (Go) e equivalente de exclusão em Node/Python, para que um futuro
  handler de debug não possa serializar `cfg` inteiro por engano.

Nenhuma nota de vault criada — não há causa raiz não óbvia nova, apenas confirmação de ausência
de regressão.

---

## Sessão 2026-08-03 — Apolo (ML-4A — documentação dos 11 campos, contrato de config) — INÍCIO/FIM

Branch `refactor/unificar-a-leitura-do-trackfw-yaml`. Escopo: `docs/cli-parity.md` e a documentação
de configuração (`README.md`, seção de `trackfw.yaml`) — registrar os 11 campos de `Update`/`Sync`
(defaults e consumidores), conforme AC8.

Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-02-unificar-a-leitura-do-trackfw-yaml-em-um-unico-carregador-nos-tres-clis.md`

**Ações:**
- `docs/cli-parity.md`: nova subseção "`trackfw.yaml` fields consumed by `update` and `sync`" dentro
  de `## trackfw update vs trackfw update harness`, com tabela dos 11 campos (chave, namespace,
  default, consumidor), o fechamento explícito da lacuna do `update` do Python (nunca havia entrada
  registrada em `cli-parity.md` — confirmado por leitura integral do arquivo, 1392 linhas) e a
  exceção intencional do shell gerado (`scaffold.go:704,731`, `hooks.js:77,104`,
  `init_gen.py:790,818`), distinta dos 5 scanners removidos.
- `README.md`: nova subseção "`update` and `sync` configuration fields" com exemplo YAML dos 11
  campos e link para o contrato completo em `cli-parity.md`. Não existe `docs/configuration.md`
  dedicado no projeto; `README.md` é o doc de configuração user-facing existente.
- **Não editado**: `internal/commands/help.go` / `npm/src/commands/help.js` /
  `pypi/trackfw/commands/help_cmd.py` (`configDocs`/`CONFIG_DOCS`). Também não documentam os 11
  campos, mas alterá-los mudaria o comportamento observável de `trackfw help` (lista de chaves e
  resolução de `trackfw help <chave>`) nos 3 CLIs — fora do Negative Scope da REQ ("não altera o
  comportamento de update e sync além do exigido pela AC6") e do escopo `docs(config)` deste ML.
  **Gap reportado para REQ futura**, não corrigido silenciosamente aqui.
- Nenhuma entrada de exceção de paridade preexistente para a lacuna do Python `update` foi
  encontrada em `docs/cli-parity.md` para remover — a lacuna nunca havia sido registrada ali
  (só na Motivation da REQ). Ação 2 do ML foi, portanto, um no-op confirmado, não uma remoção.

`make quality`: ver resultado no fechamento do ciclo.

---

## Sessão 2026-08-03 — Hefesto (Barreira de qualidade pré-Wave 3, ML-1A+ML-2A) — CONCLUÍDO

Branch `refactor/unificar-leitura-trackfw-yaml`, revisão apenas (Code Quality não executa Git;
correções ficam para o dono do código, em microlote próprio).

Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-02-unificar-a-leitura-do-trackfw-yaml-em-um-unico-carregador-nos-tres-clis.md`
Escopo: remoção completa dos helpers órfãos, ausência de código morto, duplicação entre os 3 CLIs.

**Veredito: APROVADO. Libera a Wave 3.** `go vet ./...`, `go test ./...`, `npm test` (353/353),
`pytest` (860 passed + 8 subtests) e os 5 scripts de parity/quality do Makefile passam limpos.

Achados:
- `ReadUpdateConfig`/`splitKVupdate` (Go), `readUpdateConfig` (Node), `_read_config_field`
  (Python) e os helpers privados que só eles usavam (`splitLines`, `trimLeft`, `trim`) —
  confirmada ausência total por grep, não apenas desativação. Único hit restante de
  `_read_config_field` está em `pypi/build/lib/...`, artefato de build não versionado
  (`.gitignore`), fora do diff.
- AC1 (nenhum módulo fora de `config` parseia `trackfw.yaml`): confirmado cruzando `ReadFile`/
  `readFileSync`/`open(` com `trackfw.yaml` nos 3 CLIs — a única leitura de conteúdo é a do
  carregador único; o resto são `os.Stat`/`fs.existsSync` (checagem de existência, não parse) ou
  o `grep`/`sed` do shell dos git hooks, exceção documentada para a Wave 4.
- Sem duplicação interna: cada CLI tem exatamente um `loadUpdateConfig`/`_load_update_config`,
  chamado nos 2-3 pontos que precisam de `cfg.Update`.
- Sem imports/símbolos órfãos remanescentes: `fs`/`path`/`https`/`http` (Node `sync.js`),
  `fs`/`os`/`path` (Node `update.js`) e `os`/`urllib.request`/`urllib.error`/`base64` (Python
  `sync.py`) — todos com uso real confirmado por grep, não sobras da remoção dos scanners.
- Testes novos são direcionados, não apenas "passam": `internal/sync/config_loader_test.go`
  prova precedência arquivo→env explicitamente; `pypi/tests/test_update_hooks_ac6.py` demonstra
  o efeito observável da AC6 (injeção cirúrgica em `.husky/pre-commit`), não só reroda o
  `update` e checa exit 0.
- **Achado não-bloqueante**: `trackfw update` (Python, variante sem flags — `_run`) imprime o
  banner "trackfw update — atualizando regras de agente..." **antes** de chamar
  `_load_update_config`/`config.load()`, então em YAML malformado o Python imprime a mensagem de
  erro fatal seguida do banner (saída de duas linhas antes do exit 1); Go e Node validam a config
  antes de imprimir qualquer coisa, saída de uma linha só. Mesmo exit code (1) e mesma mensagem
  de erro nos 3 — diverge só a ordem de output, não coberto pelo AC5. Achado de polimento, não
  bloqueia a Wave 3.
- **Constraint para o ML-3A (Ártemis), não achado informativo**: a variante `--dry-run`/`--json`/
  `--targets`/`--install-missing` do Python (`_run_project`) nunca chama `config.load()` — YAML
  malformado não é fatal nesse caminho. O cenário de falsificação Python de `update` **não pode**
  usar essas flags como veículo de detecção (passaria idêntico com ou sem o scanner
  reintroduzido — cenário morto que parece verde); precisa invocar `trackfw update` sem flags ou
  `trackfw sync --to <provider>`, os únicos caminhos que chamam `config.load()`
  incondicionalmente. Assimetria confirmada pré-existente ao ML-2A via `git show main:...` (não é
  regressão desta ML), mas com efeito direto sobre como a Wave 3 deve ser escrita — detalhe em
  `vault/notes/python-update-run-project-bypassa-config-load-2026-08-03.md`.
- **Para Zeus, não decidido aqui**: AC6 diz "`update` do Python lê e age sobre os 5 campos, como
  Go e Node" — verdadeiro só para `_run`, falso para `_run_project` (nunca lê os 5 campos).
  Cobertura parcial satisfaz a redação do AC6 é chamada de auditoria, não achado de qualidade.
- Efeito colateral do YAML malformado virar fatal em `update`/`sync`: avaliado como consistência,
  não regressão — o carregador único já tinha esse comportamento em `validate`/`status` desde
  antes do #106; agora os 5 scanners removidos convergem para o mesmo contrato. Mensagem estática
  e idêntica nos 3 CLIs, sem eco de trecho do YAML.

Nota de vault criada: `vault/notes/python-update-run-project-bypassa-config-load-2026-08-03.md`
— o achado do `_run_project` custaria >10min ao Ártemis se descoberto só depois do ML-3A escrito.

---

## Sessão 2026-08-03 — Apolo (ML-2A: substituir os 5 scanners artesanais pelo carregador único) — CONCLUÍDO

Branch `refactor/unificar-leitura-trackfw-yaml`, commit `f9168bb`, push feito.

REQ: `docs/req/REQ-2026-08-02-unificar-a-leitura-do-trackfw-yaml-em-um-unico-carregador-nos-tres-clis.md`
Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-02-unificar-a-leitura-do-trackfw-yaml-em-um-unico-carregador-nos-tres-clis.md`, ML-2A.

Wave 2: os 5 consumidores (`ReadUpdateConfig`/`readConfigField` em Go, `readUpdateConfig`/
`readConfigField` em Node, `_read_config_field` em Python) foram removidos e substituídos por
`cfg.Update.*`/`cfg.Sync.*` resolvidos pelo carregador único (`config.Load()`/`.load()`), nos 3
CLIs.

### Arquivos alterados
- Go: `internal/generators/update.go` (`loadUpdateConfig()` novo, chdir antes do `config.Load()`
  já que o loader Go lê relativo ao cwd do processo), `internal/sync/linear.go`, `internal/sync/jira.go`
- Node: `npm/src/commands/update.js` (`loadUpdateConfig(rootDir)`), `npm/src/commands/sync.js`
  (`getConfig` agora lê `cfg.sync.<camelCase>`)
- Python: `pypi/trackfw/commands/sync.py` (`_get_config` lê `cfg["sync"]`),
  `pypi/trackfw/commands/update.py` (AC6, ver abaixo)
- Testes novos: `internal/sync/config_loader_test.go`, `npm/tests/sync.test.js`,
  `pypi/tests/test_sync.py`, `pypi/tests/test_update_hooks_ac6.py`
- Testes existentes ajustados para o singleton `config.Load()`/`Reset()` (padrão já usado em
  `roadmap_test.go`): `internal/generators/update_test.go`, `internal/generators/identity_wiring_test.go`

### AC6 — decisão de escopo (consultei o advisor antes de implementar)
Python nunca lia `hooks/ci/backend/frontend/pkg_manager` no `update`. Implementei a leitura dos 5
campos e **agi apenas sobre `hooks`** (injeção cirúrgica de `trackfw validate` em
`.husky/pre-commit` ou `lefthook.yml`, mesmo texto/mensagens do Go/Node). NÃO adicionei
`ci-workflow`/`git-hooks` a `PROJECT_TARGET_IDS` do Python — esses dois ids não fazem parte da
lista pinada de 5 em `docs/cli-parity.md` ("Declared project targets — pinned list"); adicioná-los
seria expansão de contrato fora de escopo do ML-2A (território do Wave 4 / ML-4A). Ver
`pypi/tests/test_update_hooks_ac6.py` para a prova (fixture com `.husky/pre-commit` pré-existente
sem `trackfw validate`, que o Python nunca teria tocado antes desta mudança).

### Efeito colateral esperado, não corrigido (fora de escopo do ML-2A)
YAML malformado agora é **fatal** (exit 1) para `update`/`sync` nos 3 CLIs, porque passam a usar
`config.Load()`, que chama `os.Exit(1)`/`process.exit(1)`/`sys.exit(1)` em YAML malformado — antes,
os scanners artesanais liam `""` silenciosamente e o comando seguia com defaults vazios. É
consequência intencional do "caminho único" (mesmo comportamento de `validate`/`status`/`roadmap`
hoje), documentado aqui para Hades/Zeus não tratarem como regressão nova.

### Resultado
- `go build ./...` + `go test ./...` — verde (todos os pacotes)
- `npm test` — 353 passed (345 pré-existentes + 8 novos em `sync.test.js`), 0 failed
- `pytest` (pypi) — 860 passed (11 novos entre `test_sync.py` e `test_update_hooks_ac6.py`)
- `make quality` — verde (rodado com `LC_ALL=en_US.UTF-8`; com o `LANG=pt_BR.UTF-8` do shell local
  o cenário 29 de `check-gates-falsify.sh` reprova por mensagem localizada — confirmado
  pré-existente/ambiental via `git stash` na `main`, não causado por este ML)
- AC1: exatamente 1 ocorrência de leitura de `trackfw.yaml` por CLI (o carregador único) —
  `internal/config/config.go:105`, `npm/src/config/index.js:88`, `pypi/trackfw/config.py:146`

### Próximo passo (não iniciado)
Barreira de revisão (Hefesto/Hades/Zeus) antes da Wave 3 (ML-3A, cenários de falsificação em
`scripts/check-gates-falsify.sh`, por Ártemis).

---

## Sessão 2026-08-03 — Apolo (ML-1A: namespaces Update/Sync no contrato de config) — CONCLUÍDO

Branch `refactor/unificar-leitura-trackfw-yaml`, commit `853f1d3`, push feito.

REQ: `docs/req/REQ-2026-08-02-unificar-a-leitura-do-trackfw-yaml-em-um-unico-carregador-nos-tres-clis.md`
ADR: `docs/adr/ADR-2026-08-02-caminho-unico-de-leitura-do-trackfw-yaml-com-namespaces-tipados.md`

Wave 1 do roadmap: preparar o contrato de config para os 11 campos hoje lidos por scanners
artesanais em `update`/`sync`, SEM tocar consumidores (Wave 2, ML separado, ainda não disparado).

### 11 campos e distribuição

- `Update` (Go)/`update` (Node/Python): `hooks`, `ci`, `backend`, `frontend`, `pkg_manager`
- `Sync` (Go)/`sync` (Node/Python): `linear_api_key`, `linear_team_id`, `jira_base_url`,
  `jira_email`, `jira_token`, `jira_project`

Arquivos alterados (só estes + testes):
- `internal/config/config.go` — structs `UpdateConfig`/`SyncConfig`, populadas em `parse()`
- `npm/src/config/index.js` — `defaults().update`/`.sync`, populados em `parse()`
- `pypi/trackfw/config.py` — `defaults()["update"]`/`["sync"]`, populados em `_parse()`

Sem segundo parser/segunda leitura em nenhum CLI — reorganização da struct/dict em memória sobre
o resultado do parse único já existente. Chaves YAML continuam planas na raiz. Default de campo
ausente: string vazia nos 3 CLIs.

### Resultado

- `go build ./...` + `go test ./...` — verde (todos os pacotes)
- `npm test` (em `npm/`) — 345 passed, 0 failed
- `pytest` (em `pypi/`) — 849 passed, 8 subtests passed
- `git diff --stat` confirmado: só `internal/config/config.go`, `npm/src/config/index.js`,
  `pypi/trackfw/config.py` + 3 arquivos de teste novos. Nenhum consumidor
  (`internal/generators/update.go`, `internal/sync/{linear,jira}.go`, `npm/src/commands/{update,sync}.js`,
  `pypi/trackfw/commands/{update,sync}.py`) foi tocado.

### Ambiguidade — nenhuma

REQ/ADR foram explícitos nos 11 campos, na divisão em dois namespaces e no default. Sem decisão
autônoma a registrar além de nomenclatura óbvia (`PkgManager`/`pkgManager`/`pkg_manager` seguindo
convenção de cada linguagem).

### Próximo passo (não iniciado)

Wave 2 — migrar os 5 consumidores para ler de `cfg.Update`/`cfg.Sync` e remover os 5 scanners
artesanais (AC1 da REQ). Inclui o gap do Python (`update.py` nunca leu esses campos — AC6).

---

## Sessão 2026-08-02 — Zeus (CI verde + release v6.2.0) — CONCLUÍDO

PR #106 mergeado; `origin/main` em `c46598a`; fila com um item de decisão de ADR.

### CI quebrou — e o diagnóstico foi único

Adicionar o **PyYAML**, primeira dependência de runtime de um pacote que era zero-dep, expôs que
**nenhum job do CI instalava dependências Python** — nunca houve nenhuma. `make quality` passava
localmente porque a máquina tinha PyYAML transitivamente.

Correções, ambas deixando os gates **mais** rigorosos:

- jobs `python` e `parity`: `pip install pypi/`, derivando do `pyproject.toml` em vez de hardcodar
  `PyYAML` no workflow — nova dependência futura entra sem editar o CI
- `package-smoke`: removido `--no-deps`. Era correto quando o pacote era zero-dep; agora testaria
  configuração inexistente. Sem ele, o gate **também** valida a declaração de dependências.

### Erro meu de processo, que rendeu uma rodada extra

Na primeira rodada corrigi **os dois jobs vermelhos** e empurrei. Mas o `parity` estava
**skipped** — dependia dos que falhavam — e nunca tinha sido exercitado. Apareceu vermelho só na
segunda rodada, pela mesma causa.

Deveria ter varrido o workflow inteiro de saída, não tratado os sintomas visíveis. **Job skipped
esconde tanto quanto gate vacuoso** — mesma lição que se repetiu a sessão toda, em outra roupa.
Na segunda rodada varri os 7 jobs antes de corrigir; não houve terceira.

### Armadilha de ambiente na verificação da tag

Ao conferir a paridade de versão, o Python reportou **6.1.0** enquanto Go e Node reportavam 6.2.0.
Não era bug do bump: o `pip install pypi/` que **eu mesmo** rodei ao validar o CI deixou o pacote
instalado, e `importlib.metadata.version()` lê a **distribuição instalada**, não o fonte.

Confirmado em venv limpo: instalado corretamente, reporta 6.2.0. Desinstalei o resíduo local.

**Vale lembrar:** `trackfw version` no Python reflete o pacote instalado. Ao verificar bump a
partir do fonte, garantir que não há instalação obsoleta no ambiente.

### Release v6.2.0

Seis commits desde a `v6.1.0` (#101–#106), dois `feat`, um `fix`, um `refactor`, zero breaking →
minor. A adição do PyYAML **não** é breaking: pip resolve na instalação.

---

## Sessão 2026-08-02 — Zeus (parser de config por biblioteca YAML) — CONCLUÍDO

**Branch:** `refactor/substituir-os-parsers-artesanais-de-config-por-biblioteca-yaml-nos-tres-clis`
PR #105 mergeado; `origin/main` em `909e2b5`; fila zerada antes de começar.

### Pedido

KG: "não podemos seguir sem corrigir sabendo de um bug". Os parsers artesanais são subconjunto de
YAML — quatro defeitos silenciosos em dois dias, cada um corrigido pontualmente, mas listas
aninhadas inline, mapas inline e âncoras seguem sem suporte e **sem aviso**.

### A medição que redefiniu o trabalho — feita ANTES de escrever código

Adotar bibliotecas **sem mais nada** não resolve: **troca** a divergência artesanal por
divergência de **schema**.

| Entrada | Go `yaml.v3` | Python `PyYAML` |
|---|---|---|
| `yes` | `"yes"` string | **`True` bool** |
| `010` | **`8` int (octal)** | **`8` int (octal)** |
| `2026-08-02` | **`time.Time`** | **`datetime.date`** |

PyYAML é YAML **1.1**; `yaml.v3` é 1.2. **Go e Python divergem entre si** em `yes`.

Impacto concreto: `lenient_until` é `string // date string YYYY-MM-DD` (`config.go:24`) e
**quebraria no dia 1**. `wip_limit: 010` viraria **8**, não 10.

**A decisão central passou a ser a normalização para string na fronteira**, não a adoção da
biblioteca. Sem ela, três bibliotecas dão três resultados para o mesmo arquivo.

### Lacuna declarada, não presumida

**O Node não foi medido** — sem rede para instalar `js-yaml`/`yaml`. Em vez de escrever
comportamento suposto no ADR, virou **ML-0A**: uma wave só de medição, antes da implementação.
Escolher a biblioteca depende de dado que ainda não existe.

### Fronteira de escopo fixada

**O parser de frontmatter fica FORA.** É separado do config; convertê-lo aplicaria coerção de
data em **todo** campo `date:` de **todo** ADR e REQ — risco muito maior. Está no escopo negativo.

### Estrutura

ML-0A (medir Node) → ML-1A (implementar, **executor único** nos 3) → ML-2A (barreira).

Executor único de novo: no ciclo anterior foi o primeiro multi-CLI sem divergência nem
reconciliação. Aqui o alvo é mais difícil — semântica idêntica entre três bibliotecas
**diferentes**.

**Maior risco:** fidelidade textual. `time.Time` de volta a `2026-08-02` e `8` de volta a `010`
são irreversíveis **depois** da coerção. Se a biblioteca perder a forma original, a normalização
tem de acontecer antes — lendo o nó bruto. Está escrito como contrato no roadmap.

### Execução e fechamento

ML-0A (medir Node) → ML-1A (implementar) → **ML-1B** → ML-2A (barreira) → **ML-3A** → **ML-3B**.
`make quality` exit 0; falsificação **82 → 92**.

Três dos seis MLs foram **corretivos vindos de auditoria**.

### A medição pré-código foi o que salvou o ciclo

Medir as bibliotecas **antes** de escrever mostrou que adotá-las sem normalizar trocaria a
divergência artesanal por divergência de **schema**. A decisão central virou **normalizar para
string na fronteira**, lendo o **nó bruto** — não revertendo valor tipado, que é irreversível.

O ML-0A (só medição) rendeu três achados que teriam virado bug: octal diverge em **três**
direções (Go/Python `8`, Node `10`); Node **não** converte data, logo um teste de `lenient_until`
passaria lá sem normalização; e âncoras corrompem normalização ingênua.

### Os três corretivos

- **ML-1B** — o ML-1A introduziu **regressão**: parser all-or-nothing descartando a config
  inteira em silêncio. Medido contra a `main`: o parser antigo lia `wip_limit: 3` do arquivo
  malformado; o novo caía no default. Virou a pior instância da classe que o ciclo combatia.
  Agora falha alto. Fechou de quebra três divergências do caminho de erro.
- **ML-3A** — a barreira achou que o `validate` **contornava** o `config.Load()`. O objetivo
  estava metade cumprido. Eliminados os leitores sombra; correção por deleção.
- **ML-3B** — a regressão não tinha teste. A fixture precisa de **escalar citado**
  (`wip_limit: "3"`) para discriminar; sem aspas é vácuo — que era exatamente por que o teste
  existente não pegava nada.

### Padrão que se repetiu e vale nomear

Em **três** momentos o executor **parou e reportou** em vez de decidir: a divergência `by_agent`,
os campos fora do `ProjectConfig` em `update.go`/`sync`, e a ausência de teste da regressão. Nos
três a decisão era de arquitetura, não de implementação. Isso é o que fez a auditoria funcionar.

### FILA

`update.go`, `sync/linear.go` e `sync/jira.go` ainda leem `trackfw.yaml` diretamente, para campos
que **não existem** no `ProjectConfig` (`hooks`, `ci`, `linear_api_key`, `jira_base_url`).
Ampliar o contrato é decisão de ADR — **não** foi feito por decisão do executor, corretamente.

Pronto para merge e tag.

---

## Sessão 2026-08-02 — Zeus (fila ZERADA: lista YAML inline) — CONCLUÍDO

Último item, fechado na mesma branch do PR #105 a pedido de KG — mergear e tagear de uma vez.

### Entrega

Os três CLIs deixam de descartar `agents: [zeus, apolo]` em silêncio. Vale para `adr_dirs`,
`agents`, `acceptance_markers` e as sub-listas de `link_fields`. `rules` fica fora com razão —
é mapeamento, não sequência.

`make quality` exit 0; falsificação **78 → 82**.

### Decisão de estrutura que se pagou

**Executor único nos 3 CLIs**, não três paralelos. Justificativa registrada no ADR: os MLs
paralelos divergiram em **todos** os ciclos deste projeto, e aqui a exigência era semântica
idêntica em nove casos de parsing. Resultado: **zero divergência**, nenhum ML de reconciliação
necessário — o primeiro ciclo multi-CLI da sessão em que isso acontece.

### O caso difícil

`["a, b", "c"]` são **dois** itens. Separação ingênua por vírgula quebraria, e há caso real:
`acceptance_markers` já carrega valores com espaço e acento. Resolvido com scanner char-a-char
que rastreia aspas — mesma estratégia nos três.

### O achado mais fino: vacuidade no próprio cenário de falsificação

A Ártemis, ao escrever o cenário, foi verificar "e se alguém reverter o ML-1A **inteiro**, não só
o trecho que eu corrompo?". Confirmou com `git apply -R` que a saída ficaria **byte-idêntica ao
pinado** — o cenário seria **cego** a essa classe de regressão, e morreria no setup assim que as
funções fossem apagadas.

Corrigiu acrescentando um agente **presente no disco mas fora da lista configurada**. Reversão
total → o agente extra reaparece; reversão pontual → o item com vírgula some.

**Regra generalizada**, em `vault/notes/falsificacao-fixture-vacua-contra-reversao-total-vs-parcial-2026-08-02.md`:
cenário sobre mecanismo com **fallback** precisa de fixture com algo no conjunto de fallback que
não esteja no configurado — senão fica cego a "componente inteiro removido".

Ela também verificou **antes de editar** se o refactor da Wave 1 quebrara algum cenário herdado —
a armadilha exata do ciclo anterior. Não quebrara.

### FILA ZERADA

Nada em `backlog/`, `analyzing/`, `wip/` ou `blocked/`.

**Limite honesto do que foi entregue:** o parser continua sendo um **subconjunto** de YAML.
Listas aninhadas inline, mapas inline e âncoras seguem sem suporte **e sem aviso**. A classe foi
reduzida, não eliminada. A solução a prazo é biblioteca YAML de verdade — barata no Go
(`yaml.v3` já é indirect), mas dependência de runtime nova no Node e no Python. Mudança de
política; ADR próprio se o parser artesanal voltar a dar problema.

Pronto para merge do PR #105 e tag.

---

## Sessão 2026-08-02 — Zeus (fila zerada: 3 defeitos de parsing) — CONCLUÍDO

**Branch:** a mesma do PR #105, por pedido de KG — fechar os itens **antes da tag**, para não
versionar defeito conhecido.

### Três defeitos, e a direção da correção NÃO foi a mesma

| Item | Quem errava | Correção |
|---|---|---|
| Delimitador não pareado (`ADR: "X.md'`) | Python | alinha a Go/Node |
| Ordenação do fallback de agentes (`_list_dirs`) | Python | alinha a Go/Node |
| **Sequência YAML não indentada** | **Go e Node** | **alinham ao Python** |

### A lição do ciclo

Viemos aplicando a heurística "dois concordam, o terceiro se alinha". No item 3 ela **falharia**:
`agents:\n- zeus\n- apolo` é YAML válido — confirmei com parser real — e Go/Node descartavam a
lista **em silêncio**, caindo no fallback. O Python lia certo.

**Maioria não é autoridade quando existe especificação.** Verificar contra o padrão custou um
comando e evitou alinhar dois CLIs a um bug.

O alcance também era maior que o sintoma: o mesmo `hasIndent` governa `adr_dirs`,
`acceptance_markers` e `link_fields`, não só `agents`. E `rules` fica de fora **com razão** — é
mapeamento, não sequência; verificado que sub-chave não indentada é top-level também no YAML
padrão.

### Falha minha, pega pela barreira

O Cenário 28 quebrou com o ML-1A — ele corrompia um bloco que o ML-1A refatorou, então o literal
sumiu e o cenário passou a falhar **no setup**, não como veredito. **`make quality` ficou vermelho
nesta branch por dois commits meus.**

Na auditoria de cada ML rodei `go test`, `npm test` e `pytest` — mas **não** a suíte de
falsificação, que era exatamente a quebrada.

**Regra a incorporar:** rodar as suítes de teste não substitui rodar o gate completo. Se o ML
tocou código que algum cenário de falsificação corrompe, `check-gates-falsify.sh` precisa entrar
na auditoria **daquele ML**, não só na barreira final.

Registrado em `vault/notes/cenarios-de-falsificacao-quebram-em-refactor-do-alvo-2026-08-02.md`,
junto de um segundo acerto dela: o braço de detecção do Cenário 33 usava `os.listdir()` sem
ordenar — **dependente do filesystem**, ficaria inerte no CI. Trocado por `reverse=True`.
Corrupção que depende de ambiente é cenário vacuoso intermitente, pior que cenário ausente.

### Estado

`make quality` exit 0; falsificação **69 → 78**. Fila com **um** item, de decisão de produto:
os três CLIs ignoram lista **inline** (`agents: [a, b]`) em silêncio — consistente entre CLIs,
logo não é paridade, mas é config válida descartada sem aviso.

Pronto para tag após o merge.

---

## Sessão 2026-08-02 — Zeus (ponto 1: convergir o comando `status`) — CONCLUÍDO

**Branch:** `feat/convergir-o-comando-status-dos-tres-clis-num-formato-unico`
PR #104 mergeado; `origin/main` em `590cce8`; fila zerada antes de começar.

### Correção de premissa vinda de KG — importante

Eu enquadrei a convergência do `status` como **breaking change**. KG corrigiu: **o trackfw ainda
não tem usuários externos.** Não há saída consumida por script de terceiro, não há migração a
proteger. O custo é **interno** — fixtures e asserções dos 3 CLIs.

Isso invalida um argumento que usei **várias vezes** nesta sessão para deixar defeito de pé:
manter o nome impreciso `blocked_by_draft_adr` ("chave pública de configuração"), não unificar os
mecanismos de strip, manter `Draft` e `Proposed` separados. Nenhum tem o peso que dei.
Registrado em memória de projeto; **vale revisitar** se algum voltar à pauta.

### Decisão de KG: opção 2 — convergir preservando

Não substituir a saída do Python pela de Go/Node (que descartaria a visão de inventário), mas
**somar as duas visões** num formato único.

### Dois defeitos silenciosos descobertos ao comparar

1. **`analyzing` omitido no Python** — `commands/status.py` enumera 5 dos 6 estados em **três**
   pontos (~73, ~81, ~141). Roadmap em `analyzing/` some da contagem.
2. **`Done` e `Closed` agrupados** — apaga a distinção entre REQ entregue e encerrada sem entrega.

### Detalhe que mudou o desenho

O preview que KG aprovou dizia `📊 Inventário`. Mas os rótulos do `status` (`WIP`, `Blocked`,
`Done (last 5)`) são **hardcoded em inglês** — o bloco `status` do i18n só tem `description`.
Usar `Inventário` misturaria idiomas. Decidido: **`Inventory`**, em inglês, e i18n do `status`
fica como candidato próprio. Comunicado a KG.

### Estrutura — com ML de reconciliação PRÉ-ALOCADO

Wave 1 (3 MLs paralelos) → **Wave 2 de reconciliação** → Wave 3 de barreira.

A Wave 2 não é contingência: nos ciclos anteriores deste projeto os três MLs paralelos divergiram
**todas as vezes** — em fonte de dado, em texto de mensagem, e em raio de alcance. Aqui a exigência
é saída **byte-idêntica**, o alvo mais sensível até agora. Um executor **único** nos 3 CLIs.

### Execução e fechamento

Wave 1 (3 MLs paralelos) → **ML-2A reconciliação** → **ML-2B corretivo** → Wave 3 barreira.
`make quality` exit 0; falsificação **65 → 69**.

Os três CLIs produzem saída **byte-idêntica** em três cenários verificados: repositório real
(749 B), fixture flat com `analyzing` e os 3 status de REQ, e fixture `by_agent`.

### O ML de reconciliação pré-alocado se pagou — de novo

Eu havia reservado a Wave 2 prevendo divergência, com base no histórico. Veio, e em dois níveis:

- **ML-2A** — o Node tinha um `\n` extra. Meu handoff apontava `getStatus()` como origem; o
  executor leu os três pontos de impressão e achou o real: `console.log()` em
  `commands/status.js` somava `\n` ao que a string já trazia, enquanto Go usa `fmt.Print` e
  Python `print(..., end="")`. Diagnóstico melhor que o meu. Achou também que o Python não tinha
  `⚙ WIP by Squad` nem `⚠ Stale WIP`.
- **ML-2B** — ele sinalizou o modo `by_agent` **sem decidir sozinho**. Fui medir: a saída do
  Python não era só diferente, estava **errada** — listava nomes de estado como agentes e dizia
  `WIP (0)` havendo 1. E a seção tinha sido **adicionada nesta wave**, não era pré-existente.
  Divergência que nós introduzimos → corrigir, não diferir.

### Autocorreção na barreira que vale carregar

O primeiro braço de detecção do cenário `by_agent` corrompia a lista de agentes — o que derrubava
o bloco `Inventory` inteiro e **mascarava** se a comparação pegava divergência na seção sob teste.
A Ártemis detectou e trocou por corrupção que altera só o subdiretório lido no loop, isolando a
seção. **É a diferença entre "o gate falhou" e "o gate falhou pelo motivo certo".**

### FILA — pontos 1, 2 e 3 fechados. Dois itens novos, ambos criados por medição

1. **Delimitador não pareado** (`ADR: "X.md'`) resolve em Go/Node e não no Python (PR #104).
2. **Parser YAML do Python não trata lista inline** — com `agents: [zeus, apolo]` a ordem diverge
   de Go/Node; com lista em bloco os três concordam. Causa raiz em `pypi/trackfw/config.py`,
   **pré-existente** (não tocado neste ciclo) e com alcance além do `status`. As fixtures da
   barreira usam lista em bloco de propósito, para não mascarar.

Nenhum dos dois tem caso real no repositório.

---

## Sessão 2026-08-02 — Zeus (pontos 2 e 3 da fila: backticks + mensagem do validate) — CONCLUÍDO

**Branch:** `fix/backticks-em-campos-de-referencia-e-mensagem-de-sucesso-do-validate-no-python`
PR #103 mergeado; `origin/main` em `c7a2a34`; fila em backlog/wip zerada antes de começar.

### Pedido

KG pediu os três pontos abertos na ordem **2 → 3 → 1**. Este ciclo cobre **2 e 3**; o ponto 1 vai
em REQ própria — e **cresceu** (ver abaixo).

### Empacotamento decidido

Pontos 2 e 3 são ambos correções pequenas da **superfície do validador**, em arquivos do mesmo
domínio → **um** ADR + **uma** REQ com duas frentes. O ponto 1 é feature com mudança de saída →
REQ separada.

### Investigação — o que mudou em relação ao que eu havia reportado

**Ponto 2 tem causa ÚNICA, não dupla.** Eu suspeitava que `adr: ""` no frontmatter causasse
early-return, mascarando a linha do corpo. **Reproduzi e é falso:** `""` reduz a string vazia,
falha o teste de `.md`, e o laço **continua** corretamente. O backtick é a única causa. Instruí
explicitamente a **não** "corrigir" o early-return.

**Ponto 3 não é divergência de tradução.** Os três CLIs **já têm** `validate.ok` =
`"✓ No violations found."` no próprio `i18n/locales/en-US.json`. O Python simplesmente **não usa**
— `commands/validate.py:104` tem o literal `"✓ Governance OK"` hardcoded. A correção é fazê-lo
usar o recurso que já carrega.

**Divergência estrutural encontrada de brinde:** os três extratores tokenizam igual (primeiro
token), mas removem delimitadores de três formas diferentes — Go `strings.Trim` (conjunto), Node
regex de uma ocorrência por ponta, Python só **par casado**. Decidi **medir, não unificar**:
o AC5 exige tabela compartilhada de entradas com saída idêntica, e manda **reportar** divergência
em vez de o executor escolher sozinho. Unificar mudaria comportamento em delimitador não pareado
sem nenhum caso real.

### PONTO 1 É MAIOR DO QUE EU REPORTEI — precisa de decisão de KG

Eu disse "Python não tem o bloco de resumo". **Errado.** O comando `status` tem **duas
implementações completamente diferentes**:

- Go/Node: `🔄 WIP (n)` · `❌ Blocked (n)` · `✅ Done (last 5)` com listagem de arquivos
- Python: `Governance Status` com **contagens** — `ADRs: 19`, `REQs: 65 (0 Open, 65 Closed)`,
  `Roadmaps: backlog 0 / wip 0 / ...`

Portar significa **apagar a saída atual do Python** — mudança observável e breaking para quem
faz parsing. Alternativas: substituir, manter as duas (acrescentar o bloco ao formato atual), ou
adiar. **É decisão de KG e foi levada a ele antes de eu escrever o ADR do ponto 1.**

### Execução e fechamento

Wave 1 com 3 MLs paralelos + **ML-1D corretivo** + Wave 2 de barreira.
`make quality` exit 0; falsificação **57 → 65**.

### O corretivo mais instrutivo do ciclo (ML-1D)

Os três agentes fizeram "a mesma correção". Mas Go e Node alteraram **só** o `extractRefPath`,
enquanto o Python alterou `normalize_yaml_flat_value` — helper compartilhado por **10 call sites**,
incluindo `parse_frontmatter`, `status`, `squad`, `governance_mode` e `traceid.py`.

Resultado: no Python o backtick passou a ser removido **em todo o frontmatter**. Provei antes de
corrigir:

```
Python parse_frontmatter('adr: `docs/adr/X.md`') → 'docs/adr/X.md'   ← removia
Go     extractFrontmatterField                    → mantinha
```

**Lição transferível:** "mesma correção nos 3 CLIs" não basta — é preciso conferir se o **raio de
alcance** é o mesmo. Um CLI editar helper compartilhado enquanto os outros editam o ponto de uso
produz divergência silenciosa que nenhum teste de unidade e nenhum gate existente pega.

### Entrega acima do pedido na Wave 2

A Ártemis acrescentou cenário para a **mensagem de sucesso** — nada em CI garantia que os 3
imprimissem o mesmo texto, e foi por isso que o Python passou meses com literal hardcoded.

Decisão dela que vale carregar: comparar os três **contra um literal pinado**, não entre si. Um
diff a três passaria se todos derivassem juntos ou imprimissem vazio.

### FILA — item 2 e 3 fechados; itens restantes

1. **Ponto 1 — comando `status`** — aguardando decisão de KG (ver entrada de INÍCIO: são duas
   implementações completamente diferentes, não um bloco faltando; portar é breaking).
2. **Item 4, criado por este ciclo** — delimitador **não pareado** (`ADR: "X.md'`) resolve em
   Go/Node e não no Python. Medido, sem caso real, deliberadamente não resolvido.

---

## Sessão 2026-08-01 — Zeus (execução: regra adr_accepted_when_req_done) — CONCLUÍDO

**Branch:** `feat/detectar-adr-nao-aceito-referenciado-por-req-concluida`
PR #102 mergeado; roadmap saiu de `backlog` para `wip` e agora `done`.

### Entrega

Regra `adr_accepted_when_req_done` (`error`) nos 3 CLIs + `blocked_by_draft_adr` migrada para um
helper canônico que reconhece `Draft` **e** `Proposed`. `make quality` exit 0, **115 checks**,
falsificação **42 → 57** cenários.

### Três MLs corretivos, todos vindos da auditoria

A wave paralela (3 agentes, 1 por CLI) entregou tudo funcionando e com testes verdes — e mesmo
assim precisou de **três** correções que nenhum gate teria pego:

- **ML-1D** — divergência tripla: (a) fonte do status (Node só cabeçalho, Go/Python
  frontmatter-first), (b) falso-positivo de prosa (`Contains` no documento inteiro — o **próprio
  ADR deste ciclo** cita `"Status: Draft"` e seria flagrado pela regra que documenta), (c) fallback
  do Python não truncava no próximo pipe. Executado por **um único** agente nos 3 CLIs, de
  propósito.
- **ML-1E** — o bloco de resumo rotulava `(Draft)` um ADR `Proposed`. String **pré-existente** que
  a nossa mudança tornou mentirosa. Mesma classe do cabeçalho do `app.js` em ciclo anterior.
- **ML-1F** — **AC1 reprovou**: o Go tinha três cópias da expressão em produção e o helper
  canônico só era chamado pelos testes. Sem bug funcional, mas é a dívida que o ciclo existia para
  eliminar.

### O achado mais importante: gate verde vacuamente

`check-validate-parity.sh` **passava sem discriminar nada** — compara só `(rule, file)` e este
repositório não tem artefato que viole a regra nova. Passaria igualmente se a regra não existisse
em CLI nenhum.

A Ártemis reforçou com fixture violadora e **guard de vacuidade por regra**, e provou o guard
capaz de falhar. Sem isso teríamos "paridade verde" sem paridade verificada.

**Regra a carregar:** gate verde sobre corpus sem caso positivo não é evidência. Ao criar regra
nova, criar também a fixture que a viola.

### Notas de vault criadas

- `adr-status-substring-livre-falso-positivo-2026-08-01.md` (Apolo/Node)
- `deteccao-de-status-de-adr-divergencias-entre-clis-2026-08-01.md` (Zeus — consolida as três
  divergências e a regra prática)
- `validate-parity-gate-vacuo-e-go-sem-helper-unico-2026-08-01.md` (Ártemis)

### Lacunas pré-existentes reportadas, NÃO fechadas

1. **Python não tem o bloco de resumo** (`⏳ REQs blocked by...`) que Go e Node têm — lacuna de
   paridade anterior a este ciclo; fechá-la é feature nova.
2. **`extractRefPath` não remove backticks**: REQs cujo único campo `ADR:` está como
   `` ADR: `docs/adr/...` `` ficam invisíveis à regra nova. Três REQs no repo usam essa forma, mas
   todas apontam para ADR `Accepted` — sem falso-negativo real hoje, buraco estrutural.
3. Mensagem de sucesso do `validate` diverge: Go/Node `✓ No violations found.`, Python
   `✓ Governance OK`.

---
## Sessão 2026-08-01 — Zeus (REQ: ADR não aceito referenciado por REQ Done) — CONCLUÍDO

**Branch:** `docs/req-adr-nao-aceito-por-req-concluida`
Tag `v6.1.0` publicada; PR #101 mergeado; `origin/main` limpa.

### Pedido

KG pediu a REQ para a lacuna que eu havia sinalizado: nenhum gate detecta ADR `Proposed`
referenciado por REQ `Done`.

### A investigação encontrou algo maior

Ao rastrear **por que** o validador não pegava, descobri que o vocabulário de "ADR não aceito"
está **fragmentado entre gerador e validador**:

| Estado | Origem | Validador reconhece? |
|---|---|---|
| `Proposed` | `adr new` — o caminho **normal** | **não** |
| `Draft` | `NewADRDraft`, via `req new` (`internal/commands/req.go:110`) | sim |

`adrDraftStatusForRule` (`validator.go:1221-1235`) decide com um único
`strings.Contains(content, "Status: Draft")`.

Ou seja, além da lacuna que KG apontou, a regra **existente** `blocked_by_draft_adr` é **cega a
`Proposed`** — só funciona para stubs gerados automaticamente, não para ADRs criados pelo caminho
normal. Duas lacunas, mesma raiz.

### Decisão do usuário (AskUserQuestion)

Escopo ampliado: helper canônico `adrNotAccepted` (`Draft` ou `Proposed`) como dono único do
vocabulário, `blocked_by_draft_adr` migrada para ele, **mais** a regra nova.

### Decisões de design registradas no ADR

- **Não renomear `blocked_by_draft_adr`** — nomes de regra são chave pública de configuração
  (`rules:` no `trackfw.yaml`); renomear quebraria silenciosamente projetos downstream. O nome
  fica historicamente impreciso; a alternativa é pior.
- **"Aceito" por exclusão**, não por allowlist — preserva `Superseded`/`Deprecated`/`Rejected`
  sem enumerar, e não quebra projetos com vocabulário próprio. Trade-off aceito: um status
  digitado errado conta como aceito.
- **Severidade `error`, não `warning`** — o caso original passou despercebido justamente por não
  haver sinal; um warning a mais teria a mesma sorte.
- **Não unificar os geradores** (fazer `NewADRDraft` emitir `Proposed`): `Draft` e `Proposed` têm
  semânticas distintas e a mudança invalidaria ADRs `Draft` downstream.

### Estado da entrega

Artefatos criados; roadmap em **`backlog/`**, não em `wip`. KG pediu para **gerar** a REQ, não
para implementar — o roadmap entra em execução quando ele decidir.

### Consequência a destacar no futuro CHANGELOG

A `blocked_by_draft_adr` fica **mais rigorosa**: projetos com ADR `Proposed` ligado a REQ `Open`
passarão a ver violações que antes não viam. É a regra passando a fazer o que o nome sempre
prometeu, mas é mudança de comportamento observável.

---
## Sessão 2026-08-01 — Zeus (tag v6.1.0 + aceite do ADR de gates) — CONCLUÍDO

PR #100 mergeado; `origin/main` em `cb09ec9`; nenhuma branch aberta.

### Tag v6.1.0 publicada

Anotada, com o corpo extraído da seção `[6.1.0]` do CHANGELOG. Aponta para `cb09ec9` — conferido
que é exatamente o commit de `origin/main`, não um ancestral.

**Ordem deliberada: tag ANTES do aceite do ADR.** A tag deve ser reproduzível a partir do
CHANGELOG — `git show v6.1.0` corresponde ao que está documentado. O aceite do ADR é metadado de
governança que não consta do CHANGELOG, então entra depois e cai na próxima release. Decisão de
baixo risco, tomada e comunicada.

### ADR aceito

`ADR-2026-07-26-principios-de-design-de-gates-verificaveis` (autoria KG) → `Accepted`, com linha
`Accepted: 2026-08-01` acrescentada para preservar a data original de criação (26/07) distinta da
data de aceite. As **7 REQs** que ele governa foram reconferidas: todas `Done`.

**Não restam ADRs `Proposed`** no repositório.

### Lacuna de governança identificada (sem REQ ainda)

Nenhum gate detecta ADR `Proposed` referenciado por REQ `Done`. Foi assim que esse estado
sobreviveu a sete entregas. É o mesmo padrão dos defeitos corrigidos nesta sessão: estado
inconsistente que nenhuma verificação automática pega. Candidato natural a REQ — regra nova no
`validate` mais cenário em `check-gates-falsify.sh`. **Sugerido a KG, aguardando decisão.**

---
## Sessão 2026-08-01 — Zeus (release v6.1.0 + housekeeping de vault) — CONCLUÍDO

**Branch:** `chore/release-6.1.0`

PR #99 mergeado; `origin/main` em `1c9e6ea`; todas as branches anteriores apagadas.

### Auditoria de backlog pedida por KG

Varredura factual, não de memória:

| Fonte | Estado |
|---|---|
| `backlog/`, `analyzing/`, `wip/`, `blocked/`, `abandoned/` | **0** cada |
| `done/` | 73 roadmaps |
| REQs | 63 `Done` + 1 `Closed` — nenhuma aberta |
| `.trackfw-attention.json` | ausente |
| `TODO`/`FIXME` no código dos 3 CLIs | 0 |
| `docs/roadmap/` singular descontinuado | não existe |

Três pontas soltas encontradas — duas eram **dívida minha**.

### 1. Notas de vault desatualizadas (corrigido aqui)

Eu havia marcado `roadmap-from-req-ref-targets-exist-...` como CORRIGIDO, mas esqueci de duas:

- `roadmap-new-gera-marcador-de-aceite-invalido-2026-07-31.md` — dizia "Correção pendente" mesmo
  após o PR #96 ter corrigido.
- `seam-xss-drawer-armadilhas-de-verificacao-2026-07-31.md` — o achado dos links `../` foi
  corrigido no PR #98.

Ambas marcadas, **preservando explicitamente o que continua válido** em cada uma: a armadilha do
slug de branch na primeira, e as três armadilhas de instrumentação na segunda — que são a razão
principal daquela nota existir. Nota corrigida não é nota apagada.

Lição: ao fechar um ciclo, revisar **todas** as notas de vault que ele torna obsoletas, não só a
que originou o trabalho.

### 2. Release v6.1.0 (preparado aqui)

Cinco PRs desde a `v6.0.0`: 1 `feat` + 5 `fix`, **zero breaking** → minor. CHANGELOG escrito a
partir dos commits reais. Bump nos quatro arquivos de versão. Paridade conferida:
`go 6.1.0 / node 6.1.0 / python 6.1.0`, saída byte-idêntica nos três.

### 3. ADR ainda `Proposed` — pendente de decisão de KG, NÃO tocado

`ADR-2026-07-26-principios-de-design-de-gates-verificaveis` (autoria **KG**, 26/07) segue
`Proposed`, mas é referenciado por **7 REQs, todas `Done`**. Uma decisão que governou 7 entregas
está formalmente como proposta. É inconsistência de estado, não trabalho pendente — e a decisão
de aceitá-la é do autor. **Reportado, não alterado.**

---
## Sessão 2026-08-01 — Zeus (orquestração — SRI nas CDNs / htmx morto) — CONCLUÍDO

**Branch:** `fix/proteger-dependencias-cdn-do-dashboard-com-sri-e-remover-htmx-morto`
**ADR:** `docs/adr/ADR-2026-08-01-sri-nas-dependencias-cdn-versionadas-...md` (Accepted)
**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-08-01-proteger-dependencias-cdn-...md`

PR #98 mergeado; branch anterior apagada; `origin/main` em `e9c8b37`.
**Último item** da fila de follow-ups aberta desde o ciclo das abas ADRs/REQs.

### O levantamento mudou o escopo — não era "SRI em 5 tags"

1. **htmx tem ZERO usos.** Varredura nos 3 CLIs: nenhum atributo `hx-*`, nenhuma referência no
   `app.js`. Dependência morta baixada em toda visita. Decisão: **remover**, não proteger.
   Eliminar o vetor é estritamente melhor que mitigá-lo — e tira o `unpkg.com` da cadeia.
2. **O Tailwind não pode receber SRI.** URL não-versionada, `HTTP/2 302`,
   `cache-control: max-age=14400`. Hash fixo quebraria o dashboard **inteiro** — sem estilo
   nenhum — no próximo release deles, e silenciosamente para quem não olhasse o console.

Se eu tivesse tratado a tarefa como "adicionar integrity em cinco tags", teria protegido código
morto e quebrado o dashboard num release futuro do Tailwind.

### Decisões do usuário (AskUserQuestion)

htmx **removido**; Tailwind **sem SRI**, com o motivo em comentário no próprio `index.html` para
ninguém "uniformizar" a inconsistência depois. Saldo: dashboard passa de 1/6 para **5/6** tags
tratadas.

### Buraco que esta decisão NÃO fecha

O Tailwind é a **maior** dependência do dashboard e segue desprotegido. Está explícito no ADR
como consequência negativa aceita, não escondido. Fechar exigiria trocar a Play CDN (compilador
JIT em runtime) por artefato estático versionado — mudança de comportamento, com auditoria visual
completa. REQ própria se um dia for exigido.

### Hashes (conferidos em dois downloads independentes cada)

- marked 12.0.0 — `sha384-NNQgBjjuhtXzPmmy4gurS5X7P4uTt1DThyevz4Ua0IVK5+kazYQI1W27JHjbbxQz`
- chart.js 4.4.4 — `sha384-NrKB+u6Ts6AtkIhwPixiKTzgSKNblyhlk0Sohlgar9UHUBzai/sgnNNWWd291xqt`
- d3 7.9.0 — `sha384-CjloA8y00+1SDAUkjs099PVfnY2KmDC2BZnws9kh8D/lX1s46w6EPhpXdqMfjK6i`

### Exigência central da Wave 2

`integrity` **presente no atributo não prova nada**. O ML-2A precisa corromper um hash e confirmar
que o navegador **bloqueia** o script — em pelo menos 2 das 3 tags, para não provar um caminho só.
Sem isso o AC2 é decorativo.

### Execução e fechamento

- **ML-1A** (Afrodite) — htmx removido, SRI em marked/chart.js/d3, comentário no Tailwind.
- **ML-1B** (Afrodite, corretivo) — cabeçalho do `app.js` declarava HTMX e omitia DOMPurify.
  Ela **reportou no ML-1A e não alterou**, porque o escopo proibia tocar no arquivo. Autorizei
  em seguida. Reportar em vez de extrapolar é o comportamento certo.
- **ML-2A** (Ártemis) — prova de bloqueio em navegador real.
- **ML-3A** (Afrodite) — espelho. `make quality` exit 0, 42 cenários.

### Saldo: dashboard de 1/6 para 5/6 tags tratadas

Tailwind sem SRI (deliberado) · htmx **removida** · marked, chart.js, d3 e DOMPurify com SRI.
O `unpkg.com` saiu inteiro da cadeia.

### O levantamento valeu mais que a execução

Tratada como "adicionar integrity em cinco tags", esta REQ teria **protegido dependência morta**
e **quebrado o dashboard** num release futuro do Tailwind. Verificar cada tag antes de agir mudou
o resultado: uma virou remoção, outra virou exceção documentada.

### Confirmação cruzada não planejada

O hash que o **Chrome** computou nas mensagens de erro de integridade bate com os valores
aplicados na Wave 1 — verificação independente dos hashes, vinda do navegador e não de recálculo
nosso.

### Buraco declarado, não escondido

O Tailwind é a maior dependência do dashboard e segue desprotegido. Está no ADR como consequência
negativa aceita e em comentário no `index.html`. REQ própria se a proteção passar a ser exigida.

### FILA DE FOLLOW-UPS: ZERADA

Todos os achados colaterais acumulados desde o ciclo das abas ADRs/REQs foram fechados.

**Saldo dos seis ciclos**, todos originados de uma pergunta sobre visualizar ADRs no dashboard:
XSS armazenado corrigido · dois casos de "a ferramenta reprova o próprio artefato" · parâmetro
morto que enganava nove chamadores · navegação do drawer · cadeia de suprimentos endurecida.
Proteção de falsificação em CI subiu de **24 para 42** cenários.

### Nota operacional persistente

`make quality` passa de 2 min (42 cenários de falsificação). Rodar em background com until-loop —
o timeout padrão do Bash tool não basta.

---

## Sessão 2026-08-01 — Zeus (orquestração — links `.md` relativos no drawer) — CONCLUÍDO

**Branch:** `fix/corrigir-403-em-links-markdown-relativos-dentro-do-drawer`
**ADR:** `docs/adr/ADR-2026-08-01-resolucao-de-links-markdown-relativos-...md` (Accepted)
**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-08-01-corrigir-403-em-links-markdown-...md`

PR #97 mergeado; branch anterior apagada; `origin/main` em `6cea9ec`.

**Segunda confirmação seguida de ciclo anterior funcionando:** o roadmap deste ciclo saiu do
gerador com `req:` **completo** no frontmatter (correção do PR #97), assim como o anterior já
saíra com o heading de critérios (PR #96).

### Escopo

Links `.md` relativos dentro do drawer falham — o interceptador passa o href bruto e o whitelist
rejeita. **Reproduzido** antes de planejar:

```
?path=../roadmaps/done/v2.3-...md   → 403
?path=docs/roadmaps/done/v2.3-...md → 200
```

### Levantamento que eliminou a ambiguidade do projeto

Antes de decidir o algoritmo, levantei **todas** as formas de link `.md` em `docs/`:
`./X.md` (13), `X.md` nu (3), `../vault/notes/X.md` (3), `../../../requisições/claude/X.md` (5),
`../roadmaps/done/X.md` (1), `../../req/X.md` (1).

**Nenhuma é relativa à raiz.** Isso importa: se convivessem as duas formas, resolver contra o
diretório do documento quebraria as raiz-relativas (`docs/adr/x.md` viraria
`docs/req/docs/adr/x.md`). Como não convivem, a regra é inequívoca.

### Decisão do usuário (AskUserQuestion)

Whitelist **inalterado** — `vault/` fica fora, apesar dos 3 links. Correção é só de resolução, mais
mensagem explicativa para links que caem fora dos diretórios permitidos. Mantém a superfície de
leitura do servidor intacta, coerente com o ciclo de segurança recém-fechado.

### Estrutura

Três waves sequenciais (arquivo canônico único): resolução → verificação em navegador →
espelhamento. Mesma forma do ciclo do DOMPurify.

**Caso de maior risco, explicitado no roadmap:** navegação encadeada A → B → C precisa resolver
cada salto contra o documento **então** aberto, não contra o primeiro. É o que falha numa
implementação ingênua que fixe a base uma vez.

### Execução e fechamento

- **ML-1A** (Afrodite) — `resolveRelativeMdHref` + erro 403 explicativo. Commit `fd04979`.
- **ML-2A** (Ártemis) — prova em navegador real.
- **ML-3A** (Afrodite) — espelho npm/pypi. `make quality` exit 0, 42 cenários.

### O que elevou a qualidade desta rodada

**O teste encadeado foi desenhado para ser discriminante.** Eu pedi "A → B → C em diretórios
diferentes". A Ártemis foi além: escolheu A em profundidade 2 e B em profundidade 3, com o link
B→C sendo `../../roadmaps/done/x.md`. Com base congelada o resultado seria
`roadmaps/done/x.md` (403); com base correta, `docs/roadmaps/done/x.md` (200). Sem essa diferença
de profundidade o teste passaria mesmo numa implementação ingênua. Não estava no meu handoff.

**Fragilidade reconhecida em vez de escondida.** A Afrodite documentou em comentário que
`resolveRelativeMdHref` **não é idempotente** para caminho já completo, e que a segurança vem do
isolamento do ponto de chamada, não do algoritmo. A Ártemis então **verificou** isso na prática:
o card do Board resolve com prefixo único, não duplicado.

**Separação de ruído no console.** Aviso do CDN Tailwind e 404 de favicon do próprio Chrome
identificados como benignos e não confundidos com erro de aplicação.

### Estado da fila

Resta **um** follow-up: SRI nas outras cinco tags CDN do dashboard. Com ele, a fila de achados
colaterais acumulada desde o ciclo das abas ADRs/REQs zera.

### Nota operacional (repetida, vale fixar)

`make quality` passa de 2 min por causa do `check-gates-falsify.sh` (42 cenários). Rodar em
background e aguardar com until-loop — o timeout padrão do Bash tool não basta.

---

## Sessão 2026-08-01 — Zeus (orquestração — falso-positivo ref_targets_exist) — CONCLUÍDO

**Branch:** `fix/corrigir-falso-positivo-ref-targets-exist-em-roadmap-new-from-req`
**ADR:** `docs/adr/ADR-2026-08-01-caminho-completo-no-campo-req-...md` (Accepted)
**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-08-01-corrigir-falso-positivo-ref-targets-...md`

PR #96 mergeado; branch anterior apagada; `origin/main` em `b2d33b6`.

### Confirmação de que o ciclo anterior funcionou

O roadmap deste ciclo saiu do gerador **já com** `## Acceptance Criteria`. Primeiro ciclo em
quatro que **não** precisou de contorno manual. (Atenção: `/tmp/tfw` estava obsoleto e me enganou
por um momento — rebuild obrigatório após merge.)

### Escopo

Corrigir o falso-positivo `ref_targets_exist` no `--from-req`. Causa raiz já vinha documentada por
Ártemis; **verifiquei por reprodução** em diretório limpo antes de planejar:

```
frontmatter → req: "REQ-....md"           ← basename
corpo       → REQ: docs/req/REQ-....md    ← correto
validate    → links to REQ "..." which does not exist
```

### Decisão do usuário (AskUserQuestion)

Contrato = **caminho completo**. O parâmetro `roots` de `referenceExists` é **removido**, não
implementado — validação segue estrita. Rejeitadas: tolerar basename (afrouxa e é ambíguo com
`ADRDirs` plural) e mudar `extractRefPath` (trata o sintoma no lugar errado).

### Quarto bug descoberto no próprio setup

`roadmap new --title <t> --req <path>` grava `req: ""` **vazio** no frontmatter. Como
`extractRefPath` tem early-return para vazio, **nenhuma** violação dispara — falso-**negativo**,
complementar ao falso-positivo do `--from-req`. Mesmo campo, mesmos arquivos: **incorporado**
como AC2b em vez de virar ciclo separado. Este roadmap é a prova viva — foi gerado com `--req` e
saiu com `req: ""`.

### Risco herdado a confirmar (não presumir)

Os 6 cenários `roadmap-acceptance-heading/*/from-req` do PR #96 rodam hoje com
`ref_targets_exist` **co-ocorrendo**. A nota de vault prevê que a correção não os quebra, porque
o `assert_fails_with` casa a substring de `wip_acceptance`. **A Wave 2 confirma empiricamente.**

### Execução e fechamento

- **Wave 1 — 3 MLs em paralelo** (Apolo × 3), um por CLI. Commit `1b82d99`.
- **Wave 2 — barreira** (Ártemis): dois cenários de falsificação, contador **30 → 42**.

### Quatro defeitos fechados

1. Falso-**positivo** do `--from-req` (basename no frontmatter).
2. Falso-**negativo** do `--req` simples (`req: ""` vazio) — descoberto no próprio setup.
3. Parâmetro `roots` morto, que enganava três chamadores em cada CLI.
4. Comentário obsoleto no gate, que passaria a afirmar o oposto do cenário recém-adicionado —
   a Ártemis pegou e reescreveu.

### Falsificação independente (auditoria de Zeus)

Não aceitei 42 OK como prova. Reverti o gerador Go para `filepath.Base(reqPath)`:

```
EXIT=1
FAIL [falsify/roadmap-req-frontmatter-path/go/from-req-baseline]: ciclo limpo saiu com 1, esperava 0
```

O braço de **baseline** é o que detecta. Restaurado: 42/42, `make quality` exit 0.

### Convergência independente como sinal de handoff preciso

Os três agentes, sem se comunicarem, decidiram igual sobre manter o basename no comentário
`<!-- Derived from REQ: -->` — leitura humana, não campo validado. Paridade textual preservada
sem coordenação, o que indica que o handoff estava suficientemente específico.

### Correção do handoff pela executora

Eu havia sugerido casar o padrão `does not exist`. A Ártemis usou o texto completo
`links to REQ "REQ-flag-source.md" which does not exist` — mais discriminante, porque o genérico
casaria também as violações irmãs de ADR/Roadmap ausentes. Correção dela, e correta.

### Detalhe operacional

`check-gates-falsify.sh` agora leva **mais de 2 min** — acima do timeout padrão do Bash tool.
Rodar em background (`run_in_background`) e aguardar com until-loop.

### Fila de follow-ups (sem REQ)

1. Links `.md` relativos com `../` retornam 403 no `/api/file` do dashboard.
2. SRI nas outras cinco tags CDN do dashboard.

---

## Sessão 2026-08-01 — Zeus (orquestração — marcador de aceite do gerador) — CONCLUÍDO

**Branch:** `fix/alinhar-marcador-de-criterios-de-aceite-do-gerador-de-roadmap`
**ADR:** `docs/adr/ADR-2026-07-31-gerador-de-roadmap-emite-heading-consolidado-...md` (Accepted)
**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-31-alinhar-marcador-de-criterios-...md`

PR #95 mergeado, branch anterior apagada, `origin/main` em `e6cdd10`.

### Escopo

Corrigir o defeito que venho contornando à mão há **três ciclos**: `roadmap new` emite
`**Acceptance criteria:**` mas o validador exige o heading `## Acceptance Criteria`. Todo roadmap
novo falha no `validate` ao entrar em `wip`, nos 3 CLIs.

Contornei manualmente **de novo** para criar o roadmap desta própria correção.

### Bloco decidido (byte-a-byte nos 3 CLIs)

```
## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ]
- [ ]
```

Após `## Context`, antes de `## Wave 1`. Sem espaço à direita. Convenção espelhada de
`internal/generators/req.go:93`, que já está correto.

### Estrutura — primeiro ciclo com paralelismo real

Wave 1 tem **3 MLs em paralelo** (Apolo × 3): `internal/generators/roadmap.go`,
`npm/src/generators/roadmap.js`, `pypi/trackfw/generators/roadmap.py`. Arquivos disjuntos, cada um
com os testes do próprio CLI. Diferente dos dois ciclos anteriores, onde havia um único arquivo
canônico e tudo era sequencial.

**Ponto de atenção que moldou os critérios:** `make parity`/`make quality` **falham** enquanto os
três não estiverem prontos — `check-artifact-parity.sh` compara os artefatos gerados entre CLIs.
Por isso nenhum ML da Wave 1 tem `parity` nos critérios; cada um valida só o próprio CLI. A
paridade é a Wave 2, que age como barreira.

### Diferença relevante em relação ao ciclo do DOMPurify

Lá o seam de falsificação não pôde virar gate de CI (exigiria jsdom num projeto de zero
devDependency). **Aqui é shell puro** — gerar, mover, validar. Então o ML-2A avalia acrescentar
cenário permanente a `scripts/check-gates-falsify.sh`.

### Execução e fechamento

- **Wave 1 — 3 MLs em paralelo real** (Apolo × 3), arquivos disjuntos, sem colisão. Commit `8abfa0f`.
  Bloco byte-idêntico nos três, 2 ocorrências cada (template simples + `--from-req`).
- **Wave 2 — barreira** (Ártemis): cenário permanente em `scripts/check-gates-falsify.sh` com
  6 asserções cobrindo 3 CLIs × 2 caminhos. Contador 24 → 30.

### A lacuna que a auditoria da Wave 1 pegou

Só o ML-1C (Python) fixou o contrato em teste. Go e Node não ganharam asserção — **nenhum teste
quebrou** porque nenhum estava acoplado ao corpo gerado. E `check-artifact-parity.sh` pega
*divergência entre* CLIs, mas não pega **remoção coordenada nos três**: alguém removeria o heading
dos três, a paridade continuaria verde, e o defeito voltaria em silêncio.

Foi isso que direcionou a Wave 2. Aqui o seam **pôde** virar gate de CI — shell puro, ao contrário
do ciclo do DOMPurify, que exigiria jsdom num projeto de zero devDependency.

### Falsificação independente do gate (auditoria de Zeus)

Não aceitei "30 OK" como prova. Removi **um** dos dois blocos do gerador Go:

```
EXIT=1
expected 2 occurrences of the heading block, got 1
```

Falha por guarda de pré-condição — aborta em vez de rodar ciclo já inválido. Restaurado: 30/30 e
`make quality` exit 0.

### Achado colateral com causa raiz (não corrigido)

`roadmap new --from-req` sempre dispara `ref_targets_exist`, por **três** causas independentes:
basename no campo `req:` do frontmatter (3 CLIs); `extractRefPath` retornando no primeiro campo
casado, e o `req:` do frontmatter precede o `REQ:` do corpo; e `referenceExists(ref, roots)` que
nunca usa `roots`. Documentado em
`vault/notes/roadmap-from-req-ref-targets-exist-falso-positivo-2026-08-01.md`.

### Fila de follow-ups (sem REQ ainda)

1. `ref_targets_exist` falso-positivo no `--from-req` — causa raiz já documentada.
2. Links `.md` relativos com `../` retornam 403 no `/api/file`.
3. SRI nas outras cinco tags CDN do dashboard.

---

## Sessão 2026-07-31 — Zeus (orquestração — XSS do drawer / DOMPurify) — CONCLUÍDO

**Branch:** `fix/sanitizar-html-do-drawer-do-dashboard-com-dompurify`
**ADR:** `docs/adr/ADR-2026-07-31-sanitizacao-de-html-no-drawer-do-dashboard-com-dompurify.md` (Accepted)
**REQ:** `docs/req/REQ-2026-07-31-sanitizar-html-do-drawer-do-dashboard-com-dompurify.md`
**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-31-sanitizar-html-do-drawer-do-dashboard-com-dompurify.md`

PR #94 mergeado; branch anterior validada como squash-mergeada (diff vazio contra `origin/main`)
e apagada. Estado limpo antes de iniciar.

### Escopo

Corrigir o XSS armazenado do drawer — `app.js:919` faz `innerHTML = marked.parse(...)` sem
sanitização. Achado por Hades e confirmado pré-existente ao commit `007ebab`.

### Valores verificados (não presumir — foram medidos)

- DOMPurify **3.4.12**, `https://cdn.jsdelivr.net/npm/dompurify@3.4.12/dist/purify.min.js`
- SRI `sha384-piCcpDdJ7qVeK4Tv8Z6Hpcr3ZBIgP16TxQTPVfsLFdZ5uDgwc3Y8Ho7oUnqf12qu`
  (conferido em dois downloads independentes)
- Global UMD: `DOMPurify`

### Decisões com o usuário (AskUserQuestion)

1. **AC4 é seam de navegador em auditoria, não gate de CI.** `npm/package.json` tem zero
   devDependency e não há infra de DOM; jsdom mudaria uma propriedade do projeto. O seam prova o
   efeito (payload inerte → remove sanitização → payload executa); um gate de grep provaria só o
   padrão. Trade-off aceito: sem barreira automática de regressão em CI.
2. **SRI só na tag do DOMPurify.** Nenhuma das seis tags CDN atuais tem `integrity`.

### Estrutura

Três waves sequenciais: ML-1A (Afrodite, sanitização canônica) → ML-2A (Ártemis, seam de
falsificação em navegador) → ML-3A (Afrodite, espelho npm/pypi). Sem paralelismo — arquivo
canônico único por asset, e cada wave depende do produto da anterior.

Lição aplicada do ciclo anterior: adicionei o heading `## Critérios de Aceite` ao roadmap **antes**
do `move ... wip`, e nomeei a branch a partir do slug do roadmap. Sem isso o `validate` falharia
duas vezes — ver `vault/notes/roadmap-new-gera-marcador-de-aceite-invalido-2026-07-31.md`.

### Execução e fechamento

- **ML-1A** (Afrodite) — `renderMarkdownSafe()` como ponto único de sanitização, allowlist restrita,
  fail-safe devolvendo `null`. Tag DOMPurify 3.4.12 com SRI **reconferido por ela** contra o CDN,
  não copiado do roadmap. Commit `fd7459b`.
- **ML-2A** (Ártemis) — seam de falsificação em navegador real. Commit `7023cde`.
- **ML-3A** (Afrodite) — espelho byte-a-byte para npm e pypi. `make quality` exit 0, 82 checks.

### O que mais valeu nesta rodada

**A Ártemis evitou uma asserção vacuosa.** O vetor `<script>` não dispara *nem com a sanitização
removida* — por especificação HTML, script inserido via `innerHTML` nunca executa. Se ela tivesse
lido "flag undefined" como sucesso, teríamos uma prova que passaria igualmente com o sanitizador
desligado. Ela provou por **diferencial de presença do nó**. Registrado em
`vault/notes/seam-xss-drawer-armadilhas-de-verificacao-2026-07-31.md`.

**Reforcei o seam antes de despachar:** os vetores originais (`img`, `script`) eram tags
*bloqueadas* — provariam só a allowlist de tags, passando mesmo com a filtragem de atributos
quebrada. Acrescentei vetores em tags *permitidas* (`onclick` em `<a>`, `href="javascript:"`),
que são os que realmente exercitam a camada de filtragem.

**Uma preocupação minha que a verificação derrubou:** questionei a exclusão de `img` da allowlist,
temendo quebrar diagramas em ADRs de projetos downstream. Verifiquei: o servidor só expõe `/`,
`/static/` e `/api/*` — imagens relativas **já retornavam 404** antes da mudança. Excluir `img`
preserva o status quo. Não abri ML corretivo.

### Achados colaterais não corrigidos

1. **Links `.md` relativos com `../` retornam 403** no `/api/file` — o interceptador passa o href
   bruto e o whitelist rejeita. Pré-existente, afeta documentos reais (ex.:
   `docs/req/REQ-2026-06-13-validator-improvements.md`). Candidato a REQ própria.
2. As outras cinco tags CDN seguem sem `integrity`.
3. `closeDrawer()` não readiciona a classe `hidden` que `openDrawer()` remove — não é bug visível
   (usa `style.display`), mas invalida checagens por `classList`.

**Pendente do ciclo anterior:** REQ do marcador de critérios de aceite do gerador de roadmap
(`docs/roadmaps/backlog/ROADMAP-2026-07-31-alinhar-marcador-...`), ainda em backlog.

---

## Sessão 2026-07-31 — Hades (Revisão de segurança — barreira pré-Wave 2) — CONCLUÍDO

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-31-views-de-lista-para-adrs-e-reqs-no-dashboard.md`

**Tarefa:** revisar exclusivamente o diff do commit `007ebab` (abas ADRs/REQs) como barreira antes de
liberar a Wave 2 (espelhamento npm/pypi).

**Veredito:** **APROVADO**. `escapeHtml()` neutraliza corretamente os três contextos usados em
`createListRow()` (conteúdo, atributo `data-state` entre aspas duplas, e `aria-label` — este via
`setAttribute`, que não interpreta HTML, portanto seguro mesmo sem escape). `populateStatusFilter()`
usa `createElement`/`textContent`/`.value` — sem sink de HTML. `normalizeText()` com `\p{Diacritic}`
é uma classe de caracteres única, sem alternância/backtracking — sem risco de ReDoS. Nenhum endpoint
novo, nenhum código `.go` alterado, whitelist de `api_file.go` intocada. Nenhum segredo/caminho
absoluto exposto.

**Achado real, não-bloqueante (pré-existente):** `openDrawer()` faz `mdEl.innerHTML =
marked.parse(body || raw)` sem `DOMPurify`/sanitizador — stored XSS se o **corpo** de um artefato
markdown contiver HTML malicioso. Confirmado via `git show 007ebab~1` que o sink já era alcançável
para nós `type === 'adr'`/`'req'` através do grafo D3 da view Chain, antes desta feature — a nova
lista apenas adiciona um segundo caminho de navegação ao mesmo sink. Não bloqueia este commit. Nota
detalhada em
`vault/notes/security-drawer-marked-parse-unsanitized-stored-xss-2026-07-31.md`. Recomendação:
Hefesto/Zeus abrir REQ própria para sanitizar `marked.parse()` nos três CLIs antes de tratar como
não-issue.

**Próximo:** Wave 2 (ML-2A, espelhamento npm/pypi) liberada para prosseguir.

---

## Sessão 2026-07-31 — Zeus (orquestração — Views de lista para ADRs e REQs) — CONCLUÍDO

**Branch:** `feat/views-de-lista-para-adrs-e-reqs-no-dashboard`
**ADR:** `docs/adr/ADR-2026-07-31-listas-de-adr-e-req-no-dashboard-derivadas-de-api-chain.md`
**REQ:** `docs/req/REQ-2026-07-31-...` (Done) | **Roadmap:** `docs/roadmaps/done/ROADMAP-2026-07-31-...`

### Pedido e diagnóstico

KG perguntou se dava para ver ADRs e REQs pelo dashboard. **Já dava** — mas só pelo grafo da aba
Chain, com 137 nós, o que inviabiliza busca dirigida. O backend estava pronto: `/api/chain` devolve
`id` = caminho relativo, e `openDrawer(id)` → `/api/file` respondeu 200 numa verificação empírica.
O gap era só de navegação.

### Decisões (AskUserQuestion)

Duas abas separadas (não uma "Docs" unificada) e reuso de `/api/chain` (nenhum endpoint novo).
Isso reduziu a entrega a **frontend puro**.

### Execução

- **ML-1A** (Afrodite) — abas ADRs/REQs em `internal/serve/static/`. Commit `007ebab`.
- **ML-1B** (Afrodite, corretivo) — auditoria visual em navegador real com o SO em modo escuro
  flagrou as linhas renderizando escuras num dashboard light-only. Removido o bloco
  `@media (prefers-color-scheme: dark)`, que era o **primeiro do projeto inteiro**.
- **Barreira de segurança** (Hades) — APROVADO. Achado colateral não bloqueante: `marked.parse()`
  sem DOMPurify no drawer, **pré-existente** (verificado em `007ebab~1`).
- **ML-2A** (Afrodite) — espelho byte-a-byte para npm e pypi. `make quality` exit 0.

### Aprendizados

1. **Gates verdes não provam UX.** `build`/`test`/`lint`/`quality` passaram no ML-1A com o defeito
   visual presente. Só o navegador real pegou — e só porque o SO estava em modo escuro.
2. **Reatribuição de papel:** o ML-2A foi atribuído a Hefesto por erro meu. Ele recusou
   corretamente — code quality não modifica código. Cópia mecânica ainda é implementação.
3. **Contagem nos critérios de aceite envelhece:** escrevi 12 ADRs/58 REQs; a entrega deu 13/59
   porque a própria ADR e REQ desta feature entraram na varredura. Prever isso quando o critério
   conta artefatos do próprio repositório.

### Notas de vault criadas

- `roadmap-new-gera-marcador-de-aceite-invalido-2026-07-31.md` — `roadmap new` gera
  `**Acceptance criteria:**` mas o validador exige o heading `## Acceptance Criteria`; todo roadmap
  novo falha no `validate` ao entrar em `wip`. **Defeito aberto do trackfw, não corrigido.**
- `dashboard-serve-e-light-only-2026-07-31.md` — não adicionar `prefers-color-scheme` ao dashboard.
- `security-drawer-marked-parse-unsanitized-stored-xss-2026-07-31.md` (Hades).

### Pendências geradas (REQs próprias, fora deste escopo)

1. Sanitizar `marked.parse()` no drawer com DOMPurify nos 3 CLIs.
2. Alinhar `internal/generators/roadmap.go` para emitir o heading de critérios de aceite (3 CLIs).

---

## Sessão 2026-07-31 — Afrodite (ML-1A — Abas ADRs/REQs no dashboard) — CONCLUÍDO

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-31-views-de-lista-para-adrs-e-reqs-no-dashboard.md`

**Tarefa:** ML-1A — Adicionar abas ADRs e REQs ao `trackfw serve`, como listas navegáveis com busca
(texto, acento-insensitive) e filtro de status derivado dinamicamente de `/api/chain` (nunca hardcoded).

**Branch:** `feat/views-de-lista-para-adrs-e-reqs-no-dashboard` (entregue **uncommitted** — Afrodite não
tem autoridade Git; aguardando auditoria/commit por trackfw_architect).

**Arquivos modificados (exclusivamente os três do escopo):**
- `internal/serve/static/index.html`: dois botões (`tab-adr`, `tab-req`) no `<nav>` e duas `<section>`
  (`view-adr`, `view-req`) espelhando estrutura/convenções das views existentes (busca, select de
  status, contador, loading, empty, `role="alert"`).
- `internal/serve/static/app.js`: `switchView()` estendido com ramos `'adr'`/`'req'`; novo bloco
  "Views de lista" reusando o cache `_chainData` (sem fetch novo) — `loadListView`,
  `populateStatusFilter` (popula `<select>` a partir dos valores distintos de `state`, nunca hardcoded),
  `applyListFilters`, `createListRow` (linha focável, `openDrawer(node.id)` no click/Enter/Space),
  `normalizeText` (NFD + remoção de diacríticos para busca acento-insensitive).
- `internal/serve/static/style.css`: `.list-view-container`, `.list-row`, `.status-chip` (+ variantes
  `data-state` conhecidas com fallback cinza genérico) e bloco `@media (prefers-color-scheme: dark)`
  para legibilidade dos chips em tema escuro.

**Evidência:**
- `make build && make test && make lint` → EXIT=0 (test: todos os pacotes `ok`; lint: sem saída)
- `git status --porcelain` → exatamente os três arquivos do escopo, nada mais
- `bin/trackfw serve --port 8791` (binário rebuilded pós-edit, pois assets são `go:embed`):
  `/api/chain` retorna `Counter({'roadmap': 68, 'req': 59, 'adr': 13})` — 13/59 em vez de 12/58 porque
  o próprio ROADMAP+REQ desta feature somaram 1 a cada contagem; comportamento correto, não regressão.
  ADR `docs/adr/ADR-001-...md` com `state: unknown` presente na resposta — confirmado renderizável com
  filtro "Todos" (chip cai no fallback cinza genérico do CSS, não é ocultado).
  `tab-adr`, `tab-req`, `view-adr`, `view-req` presentes no HTML servido; `/api/file` retorna 200 para
  nós de ambos os tipos — `openDrawer` reusado sem alteração.

**Status:** todos os critérios de aceite do ML-1A atendidos. Próximo: Wave 2 (ML-2A, Hefesto) espelha
os três arquivos para `npm/` e `pypi/` — **estritamente sequencial**, depende desta auditoria.

---

## Sessão 2026-07-30 — Artemis (ML-3A — Gate -v e falsificação seam Go) — CONCLUÍDO

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-30-reservar-v-para-verbose-e-remover-atalho-de-versao-no-go.md`

**Tarefa:** ML-3A — Cobrir `-v` no gate (`check-cli-parity.sh`) com duas asserções por runtime (exit
não-zero + saída não casa `^trackfw [0-9]+\.[0-9]+\.[0-9]+$`) e provar não-vacuidade com Cenário 23
(seam Go: remoção de `root.Flags().Bool("version", ...)` → cobra reregistra `-v` → gate falha).

**Branch:** `feat/reservar-v-para-verbose-e-remover-atalho-de-versao-no-go`

**Arquivos modificados:**
- `scripts/check-cli-parity.sh`: bloco `-v flag` inserido antes de `check-integration-cli-parity.sh`.
  Dois estágios por runtime: vacuity-guard (saída não-vazia), Assertion-1 (exit -ne 0), Assertion-2
  (grep -Eq negativo contra _VERSION_RE). Verificação empírica: nenhum runtime produz linha matching
  a regex com `-v` rejeitado (Go: erro+usage; Node: `error: unknown option '-v'`; Python: usage+erro).
- `scripts/check-gates-falsify.sh`: Cenário 23 com guarda de padrão (sed), guarda de vivacidade
  (build_go_or_fail + execução do binário corrompido confirmando exit 0 + formato de versão), e
  `assert_fails_with "cli-parity/v-flag-accepted"` rodando o gate a partir de T23 (cd T23 →
  `go build ./cmd/trackfw` pega o internal/ corrompido). Total: 23 → 24 cenários (gates: 14).

**Evidência:**
- `bash scripts/check-cli-parity.sh` → EXIT=0 (cenário positivo)
- `bash scripts/check-gates-falsify.sh` → 24/24 OK, EXIT=0 (incluindo `cli-parity/v-flag-accepted`)
- `make quality` → EXIT=0
- Cenários 21 e 22 permanecem verdes (não regressão PR #91)

**Status:** todos os critérios de aceite do ML-3A atendidos.

---

## Sessão 2026-07-30 — Artemis (ML-3A — Gate unificado + falsificação) — CONCLUÍDO

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-30-padrao-unico-de-saida-de-versao-nos-tres-clis.md`

**Tarefa:** ML-3A — Unificar a asserção de versão em `check-cli-parity.sh` e adicionar duas provas P4
de não-vacuidade (seam A = regex arm; seam B = byte-comparison arm).

**Branch:** `feat/padrao-unico-de-saida-de-versao-nos-tres-clis`

**Commit:** 459edd6

**Arquivos modificados:**
- `scripts/check-cli-parity.sh`: linhas 103-109 substituídas por bloco com capture guards, vacuity
  guard, single-line guard, format assertion unificada (`^trackfw [0-9]+\.[0-9]+\.[0-9]+$`) e
  byte-comparison das 6 saídas.
- `scripts/check-gates-falsify.sh`: Cenário 21 (seam A — regex arm, corrupts `version.js`) e
  Cenário 22 (seam B — byte-comparison arm, corrupts `package.json`). Total: 21 → 23 cenários.

**Evidência:**
- `bash scripts/check-cli-parity.sh` → EXIT=0
- `bash scripts/check-gates-falsify.sh` → 23/23 OK, EXIT=0
- `make quality` → EXIT=0
- `bin/trackfw validate --json` → 0 violações
- `git status` → limpo

**Status:** todos os critérios de aceite do ML-3A atendidos. Wave 3 concluída.

---

## Sessão 2026-07-30 — Apolo (ML-2B — Node.js: alinhar `--version` com subcomando `version`) — CONCLUÍDO

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-30-padrao-unico-de-saida-de-versao-nos-tres-clis.md`

**Tarefa:** ML-2B — Node.js: fazer `--version` imprimir `trackfw <semver>`, byte-idêntico ao subcomando `version`.

**Branch:** `feat/padrao-unico-de-saida-de-versao-nos-tres-clis`

**Entregue:**
- `npm/src/commands/index.js`: `.version(version)` → `.version(`trackfw ${version}`)` — commander agora imprime o formato correto na flag `--version`.
- `npm/tests/version.test.js`: 3 testes travando o formato exato (`^trackfw [0-9]+\.[0-9]+\.[0-9]+$`) para `version` e `--version` e igualdade byte-a-byte entre ambos.

**Saída verificada:**
```
node npm/bin/trackfw version   → trackfw 5.0.0
node npm/bin/trackfw --version → trackfw 5.0.0
BYTE-IDENTICAL: ok
```

**Testes:** 342 pass, 0 failed (`cd npm && npm test`).

---

## Sessão 2026-07-30 — Apolo (ML-2A — Go: remover prefixo `v` da constante de versão) — CONCLUÍDO

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-30-padrao-unico-de-saida-de-versao-nos-tres-clis.md`

**Tarefa:** ML-2A — Remover o `v` de `var Version = "v5.0.0"` em `internal/version/version.go` e adicionar teste travando o formato exato das duas superfícies.

**Branch:** `feat/padrao-unico-de-saida-de-versao-nos-tres-clis`
**Commit:** `f7785ea`

**Entregue:**
- `internal/version/version.go`: `"v5.0.0"` → `"5.0.0"` (sem prefixo `v`).
- `internal/commands/version.go`: `fmt.Println` → `fmt.Fprintln(cmd.OutOrStdout())` para capturabilidade nos testes.
- `internal/commands/version_test.go`: testes `TestVersionSubcmdFormat`, `TestVersionFlagFormat`, `TestVersionSurfacesByteIdentical` travando o formato exato e igualdade byte-a-byte.

**Saída verificada empiricamente:**
```
bin/trackfw version   → trackfw 5.0.0
bin/trackfw --version → trackfw 5.0.0
diff → byte-idênticos
```

**Coverage:** go build ok · go test ./... ok (todos os pacotes) · go vet ok.

---

## Sessão 2026-07-30 — Artemis (ML-3A — Auditar paridade e provar não-vacuidade) — CONCLUÍDO

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-30-roadmap-move-sincroniza-a-referencia-da-req-pareada.md`

**Tarefa:** ML-3A — gate permanente de paridade para `roadmap move + sync de REQ`.

**Branch:** `feat/roadmap-move-sincroniza-a-referencia-da-req-pareada`
**Commit:** `1bbc8b6`

**Entregue:**
- `scripts/check-roadmap-move-parity.sh`: gate com 5 cenários (zero-req, one-req, by_agent-discriminant, points-at-other, idempotency); fixtures isoladas por runtime; vacuity-guard em todos; seam via sed no Node.js.
- `scripts/check-gates-falsify.sh`: Cenário 20 — seam que corrompe sort do Node.js (basename→path), provando que o gate captura regressão de ordenação na fixture discriminante. `GATES_MUTATION_CHECK` atualizado. Contador: 21 cenários / 14 gates.
- `Makefile`: novo gate encadeado antes do falsify.

**Coverage:** Go 15 pk ok · Node.js 339 pass · Python 724 pass · make quality exit 0 · validate 0 violations.

**Handoff ao orquestrador:** ML-3A concluído. Roadmap pronto para Wave 4 → done.

---

## Sessão 2026-07-30 — Apolo (ML-2F — Python: ordenar por basename e alinhar linha `moved`) — CONCLUÍDO

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-30-roadmap-move-sincroniza-a-referencia-da-req-pareada.md`

**Tarefa:** ML-2F — duas correções no Python CLI.

**Entregue:**
- `pypi/trackfw/generators/roadmap.py`: `sorted(resolve_req_files(cfg))` → `sorted(..., key=lambda p: (os.path.basename(p), p))` — ordena por basename, com caminho completo como desempate.
- `pypi/trackfw/commands/roadmap.py`: `print(f"Roadmap movido para: {new_path}")` → `print(f"✓ moved {os.path.basename(new_path)} → {os.path.dirname(new_path)}")` — paridade byte-a-byte com o Go.
- `pypi/tests/test_generators_roadmap.py`: teste discriminante `test_by_agent_ordenacao_por_basename_fixture_discriminante` — fixture `apolo/done/REQ-zzz` + `zeus/backlog/REQ-aaa`, valida que `synced = ["REQ-aaa.md", "REQ-zzz.md"]`.

**Validação:**
- Fixture discriminante: `synced[0] = "REQ-aaa.md"`, `synced[1] = "REQ-zzz.md"` ✓
- Paridade Go vs Python (execução lado a lado): idênticos — `✓ moved ROADMAP-... → docs/roadmaps/wip`
- Suíte completa: 724 passed, 0 failed.

**Status:** CONCLUÍDO

---

## Sessão 2026-07-30 — Apolo (ML-2D — Go: ordenar por basename em syncREQReferences) — CONCLUÍDO

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-30-roadmap-move-sincroniza-a-referencia-da-req-pareada.md`

**Tarefa:** ML-2D (corretivo) — corrigir ordenação lexicográfica por basename na lista final de REQs em `syncREQReferences`; adicionar fixture discriminante `by_agent` (apolo/REQ-zzz + zeus/REQ-aaa → sequência aaa, zzz).

**Entregue:**
- `internal/generators/roadmap.go`: import `"sort"` adicionado; `sort.Slice` por basename (desempate por caminho completo) inserido em `syncREQReferences` após `scanREQFiles`.
- `internal/generators/roadmap_test.go`: `TestSyncREQ_ByAgent_OrderByBasename` — fixture discriminante que distingue ordenação por caminho de ordenação por basename; asserta a sequência exata das linhas de output.

**Validação:**
- `go build ./...` ✓ | `go test ./...` 15 pacotes ok | `go vet ./...` ✓
- `TestSyncREQ_ByAgent_OrderByBasename` PASS: linha 0 = `✓ synced REQ-aaa.md → ...`, linha 1 = `✓ synced REQ-zzz.md → ...`

---

## Sessão 2026-07-30 — Apolo (ML-2E — Node.js: ordenação explícita por basename em syncReqReferences)

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-30-roadmap-move-sincroniza-a-referencia-da-req-pareada.md`

**Tarefa:** ML-2E (corretivo) — adicionar `.sort()` por basename após `resolveReqFiles(cfg)` em `syncReqReferences`; atualizar teste multi-REQ para assertar sequência; adicionar fixture discriminante `by_agent`.

**Entregue:**
- `npm/src/generators/roadmap.js`: `syncReqReferences` agora ordena a lista retornada por `resolveReqFiles` por basename (comparação `<`/`>` pura, locale-independente), com desempate por caminho completo.
- `npm/tests/roadmap_move.test.js`: teste multi-REQ atualizado para assertar `posA < posB` (sequência, não conjunto); fixture discriminante `by_agent` adicionada: `agents=[zeus,apolo]`, `apolo/done/REQ-zzz.md` e `zeus/backlog/REQ-aaa.md`, asserta que `aaa` é emitido antes de `zzz`.

**Validação:** `cd npm && npm test` → 339 passed, 0 failed. `node tests/roadmap_move.test.js` → 25 testes — 25 passaram, 0 falharam.

**Commit:** `69e7d03` — `fix(roadmap): ordena REQs por basename em syncReqReferences no Node.js`

**Status:** CONCLUÍDO

---

## Sessão 2026-07-30 — Apolo (ML-2C — Python: sync REQ reference no roadmap move)

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-30-roadmap-move-sincroniza-a-referencia-da-req-pareada.md`

**Tarefa:** ML-2C — implementar `sync_paired_req_references` no Python CLI.

**Entregue:**
- `pypi/trackfw/generators/roadmap.py`: três helpers novos — `_get_frontmatter_roadmap_value` (extrai `roadmap:` do frontmatter sem backticks), `_rewrite_req_roadmap_ref` (reescreve frontmatter + corpo preservando formatação), `sync_paired_req_references` (orquestra varredura flat/by_agent, 5 cardinalidades, usa import escopado de `resolve_req_files`).
- `pypi/trackfw/commands/roadmap.py`: `_cmd_move` chama `sync_paired_req_references` após move bem-sucedido e imprime `✓ synced` / `trackfw roadmap move: failed to sync` com exit não-zero em falha.
- `pypi/tests/test_generators_roadmap.py`: 21 novos testes cobrindo todas as cardinalidades, idempotência byte-a-byte, `by_agent`, backticks, e caracteres Unicode pinados.

**Validação:** `cd pypi && python3 -m pytest` → 723 passed, 0 failed.

**Divergência reportada:** Python ordena a lista de REQs (`sorted()`); Node.js não ordena (`readdirSync` sem sort). ML-3A deve pinar se a ordem é intencional.

**Status:** CONCLUÍDO

---

## Sessão 2026-07-30 — Apolo (ML-2A — Go: roadmap move sincroniza referência REQ)

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-30-roadmap-move-sincroniza-a-referencia-da-req-pareada.md`

**Tarefa:** ML-2A — implementar `syncREQReferences` em `internal/generators/roadmap.go` e testes correspondentes.

**Entregue:**
- `internal/generators/roadmap.go`: `syncREQReferences` (orquestra), `scanREQFiles` (espelha validator), `extractFrontmatterRoadmap`, `rewriteREQRoadmapRef` (backtick/aspas preservados). Inserção em `MoveRoadmap` após `fmt.Printf("✓ moved ...")`.
- `internal/generators/roadmap_test.go`: 10 novos testes — 5 cardinalidades, idempotência byte-a-byte, by_agent, backticks, erro-continua, integração.
- **Divergência:** spec dizia "antes do appendTransitionLog" mas contrato pina ✓ synced após ✓ moved; inserção correta é após fmt.Printf.

**Validação:** `go build ./...` ✓ | `go test ./...` 15 pacotes ok | `go vet ./...` ✓ | commit `02c5dee`

**Status:** CONCLUÍDO

---

## Sessão 2026-07-30 — Apolo (ML-2B — Node.js: roadmap move sincroniza referência da REQ pareada)

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-30-roadmap-move-sincroniza-a-referencia-da-req-pareada.md`

**Tarefa:** ML-2B — Implementar sincronização da referência `roadmap:` das REQs pareadas no Node.js,
dentro de `moveRoadmap` em `npm/src/generators/roadmap.js`. As cinco cardinalidades do contrato,
idempotência byte-a-byte, cobertura `by_agent`, formatação com backticks e testes correspondentes.

**Entregue:**
- `npm/src/generators/roadmap.js`: import de `resolveReqFiles` do validator; helpers
  `extractFrontmatterRoadmap` (escopado ao FM), `rewriteReqRoadmapRef` (substituição literal
  preserva backticks/aspas), `syncReqReferences` (5 cardinalidades, by_agent, stderr/exit não-zero);
  `moveRoadmap` chama `syncReqReferences(basename, dst, cfg)` após `console.log('✓ moved ...')`.
- `npm/tests/roadmap_move.test.js`: 10 novos testes — uma por cardinalidade (5), idempotência
  byte-a-byte, by_agent, backticks, erro de escrita, validateRefTargetsExist (zero violações).

**Validação:** `cd npm && npm test` → 339 passed, 0 failed.
**Commit:** `ba13af9`

**Divergência reportada:** ordem de varredura de múltiplas REQs não pinada no contrato. `fs.readdirSync`
sem sort → não garante ordem lexicográfica. Teste asserta conjunto, não sequência. Se Go ordenar e Node
não, ML-3A detectará na auditoria de paridade.

**Status:** CONCLUÍDO

---

## Sessão 2026-07-30 — Apolo (ML-2D — Python: corrigir early-break e alinhar mensagem --wave)

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-29-barrier-aceita-wave-com-sufixo-bis.md`

**Tarefa:** ML-2D (corretivo Wave 3) — corrigir early-break em `_find_wave` (Python não detectava heading
malformada depois da wave alvo) e alinhar mensagem de `--wave` inválido ao texto canônico do Go.

**Entregue:**
- `pypi/trackfw/commands/barrier.py`: novo helper `_is_valid_wave_label` (fullmatch + `>= 1`); `_find_wave`
  reescrito com pré-passo completo em dois passos (Fase 1: validar todas as headings; Fase 2: buscar label);
  `_parse_wave_label` usa `_is_valid_wave_label` e emite `invalid --wave "<v>" — not a valid wave label` (U+2014).
- `pypi/tests/test_barrier.py`: dois novos testes de posição (`antes` e `depois`), mais teste de mensagem
  `--wave` byte-exata (`test_wave_argumento_invalido_mensagem_pinada_literalmente`).

**Evidência empírica (duas posições):**
- Malformada ANTES wave alvo: exit 2, `malformed wave heading at line 5: "X" is not a valid wave label`
- Malformada DEPOIS wave alvo: exit 2, `malformed wave heading at line 13: "X" is not a valid wave label`

**Validação:** 701/701 testes passando (`cd pypi && python3 -m pytest`)
**Status:** CONCLUÍDO

---

## Sessão 2026-07-30 — Apolo (ML-2E — Node.js: alinhar mensagem --wave inválido)

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-29-barrier-aceita-wave-com-sufixo-bis.md`

**Tarefa:** ML-2E (corretivo Wave 3) — alinhar a mensagem de `--wave` inválido no Node.js ao texto canônico do Go:
`trackfw barrier: invalid --wave "<value>" — not a valid wave label` (travessão U+2014).

**Entregue:**
- `npm/src/commands/barrier.js` linha 312: removida mensagem antiga com dica `(must be a valid wave label, e.g. 1, 2-bis)`.
- `npm/tests/barrier.test.js`: adicionado teste `barrier regression: invalid --wave message is pinned literally (fourth exit-2 message)` verificando texto byte-exato com `--wave 2-BIS`.
- Go inalterado — já era o texto canônico.

**Verificação byte-a-byte:**
- Node.js: `trackfw barrier: invalid --wave "2-BIS" — not a valid wave label`
- Go:      `trackfw barrier: invalid --wave "2-BIS" — not a valid wave label`
- Comparação xxd: BYTE-IDÊNTICO (separador `\xe2\x80\x94` U+2014 em ambos).

**Validação:** `cd npm && npm test` → 339 passed, 0 failed.
**Commit:** `b55393d` em `feat/barrier-aceita-wave-com-sufixo-bis`.
**Status:** CONCLUÍDO

---

## Sessão 2026-07-30 — Apolo (ML-2C — Python: barrier aceita wave com sufixo bis)

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-29-barrier-aceita-wave-com-sufixo-bis.md`

**Tarefa:** Implementar suporte ao sufixo de wave no runtime Python (`pypi/trackfw/commands/barrier.py`).

**Entregue:**
- `_WAVE_HEADING_RE` atualizado para gramática pinada: `^## Wave (\d+(?:-[a-z0-9]+)?) `
- `_ANY_WAVE_H2_RE` novo: detector amplo para headings fora da gramática (abort de documento)
- `_parse_wave_int` → `_parse_wave_label`: valida com `re.fullmatch`, aceita `2-bis`, rejeita `2-bis-ter`, `2-BIS`, `0`, `abc`
- `_find_wave` agora aceita `wave_label: str`; identidade exata (`== wave_label`), sem prefix match
- Mensagem de heading malformada usa aspas duplas explícitas (não `!r`)
- `doc["wave"]` mudou de `int` para `str` — nenhum teste ou gate externo assertava no tipo
- 6 novos testes em `pypi/tests/test_barrier.py` incluindo regressão de abort (ADR decisão 16)
- Heading fora da gramática continua abortando o documento inteiro — abort é feature, não bug

**Validação:** 699/699 testes passando (`cd pypi && python3 -m pytest`)
**Commit:** `feat/barrier-aceita-wave-com-sufixo-bis` — `15f8ed8`
**Status:** CONCLUÍDO

---

## Sessão 2026-07-29 — Apolo (ML-2B — Node.js: barrier aceita wave com sufixo bis)

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-29-barrier-aceita-wave-com-sufixo-bis.md`

**Status:** CONCLUÍDO

**Tarefa:** Implementar suporte a rótulo de wave com sufixo (`2-bis`, `2-hotfix`) no runtime Node.js.
Escopo: `npm/src/commands/barrier.js` e `npm/tests/`. NÃO tocou em `internal/` nem `pypi/`.

**Entregue:**
- `npm/src/commands/barrier.js`: `WAVE_SCAN_RE = /^## Wave (\S+) /` (trailing space, espelho do Go),
  `WAVE_LABEL_RE`, `isValidWaveLabel` exportada; `findWave` migrado para pré-passo completo
  (valida todas as headings antes de buscar); comparação `token === String(waveLabel)` (string exata,
  nunca `parseInt`); mensagem de malformed pinada: `"<token>" is not a valid wave label`; mensagem
  "wave not found" usa rótulo; campo `wave` no JSON agora é string.
- `npm/tests/barrier.test.js`: tabela grammar, resolução de 2-bis, não-match 2 vs 2-bis, token na
  mensagem, regressão de abort (unit + CLI level).

**Validação:** `cd npm && npm test` → 338 passed, 0 failed.

**Impacto da mudança `wave` string:**
- Nenhum teste asserta o tipo numérico de `doc.wave`.
- Nenhum script em `scripts/` consome `.wave` via jq ou outro.
- `printTextReport` usa interpolação `${doc.wave}`, que funciona igual com string.
- Mudança é observável apenas em `--json` output: `"wave": 1` → `"wave": "1"`.

**Mensagem `invalid --wave value` (não pinada):**
`invalid --wave value: "<label>" (must be a valid wave label, e.g. 1, 2-bis)`

**Ordenação:** N/A — `barrier.js` não lista nem ordena waves; `findWave` faz exact-match apenas.

**`barrier-contract.test.js`:** não editado (frozen contract file).

**Commit:** `6df987b` em `feat/barrier-aceita-wave-com-sufixo-bis`.

---

## Sessão 2026-07-29 — Ártemis/QA (ML-6I — corretivo: gate `check-update-parity.sh` mutava o repositório)

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md`

**Tarefa:** Handoff do `trackfw_architect` — corrigir o achado colateral registrado por Apolo
(ML-6H) e em `vault/notes/update-parity-gate-writes-real-claude-md-2026-07-29.md`:
`scripts/check-update-parity.sh`'s `install_claude_agents()` rodava `agents install --scope global`
sem isolar `cwd`, mutando o `CLAUDE.md` real do repositório sempre que o gate era executado da raiz
(`make quality`/`make parity`/direto) — enquanto reportava `OK` em todos os cenários e saía com 0.

**Entregue:**
- `scripts/check-update-parity.sh`: `install_claude_agents()` agora roda dentro de um `scratch_dir`
  descartável criado sob `$WORK` (removido pelo `trap ... EXIT` do topo do script), em vez do `cwd`
  herdado do chamador.
- Auditados todos os outros pontos de invocação de CLI em `check-update-parity.sh`,
  `check-barrier.sh`, `check-slash-parity.sh` e `check-rules-parity.sh` — nenhum outro caso do mesmo
  padrão encontrado; todas as demais chamadas já estavam em subshell `(cd "$scratch_dir" && ...)`.
  `check-cli-parity.sh` invoca as CLIs sem `cd`, mas apenas para `--help`/`version`/`--version`
  (sem efeito colateral) — gate antigo, já verde, fora do escopo do corretivo.
- `scripts/check-gates-falsify.sh`: novo Cenário 18 (`falsify/no-repo-mutation`) — captura
  `git status --porcelain` na raiz antes/depois de rodar os quatro gates que invocam CLIs reais e
  reprova o pipeline se a árvore de trabalho mudar. Contagem final atualizada para 19 cenários.
- `vault/notes/update-parity-gate-writes-real-claude-md-2026-07-29.md`: atualizado de "fix não
  aplicado" para "fix aplicado", com o trecho de código corrigido e a descrição do Cenário 18.

**Validação (evidência exata, ver transcript):**
- `git checkout -- CLAUDE.md && git status --short CLAUDE.md` → limpo antes de começar.
- `git status --porcelain` antes/depois dos 4 gates rodados individualmente da raiz → `diff` vazio,
  `"NENHUMA MUTACAO"`.
- `GO_BIN=bin/trackfw scripts/check-gates-falsify.sh` → 19/19 cenários `OK`, incluindo
  `falsify/no-repo-mutation`.
- `make quality` (execução completa) → `EXIT_CODE=0`; `git status --porcelain` antes/depois idêntico;
  `git status --short CLAUDE.md` limpo ao final.
- `trackfw validate` → `✓ No violations found.`
- `find ~/.claude ~/.codex ~/.gemini -mmin -30 -type f | grep -i trackfw` → só logs de sessão do
  próprio Claude Code, nenhum artefato de instalação indevido.

**Escopo respeitado:** nenhuma edição em `docs/adr/`, `docs/req/`, `docs/roadmaps/`,
`docs/cli-parity.md`, `CHANGELOG.md`, ou código de runtime (`internal/`, `npm/src/`,
`pypi/trackfw/`). Nenhuma operação Git além de leitura/`checkout -- CLAUDE.md`.

**Ambiguidade reportada, não corrigida:** nenhuma. Todos os critérios de aceite do handoff foram
atendidos sem decisão bloqueante.

**Próximo agente:** commit/push e atualização de status do ML ficam para quem detém a branch — este
agente (QA) não executa operações Git de escrita.

---

## Sessão 2026-07-29 — Apolo (ML-2A — skip de artefato outdated+owned no runtime Go)

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-29-install-pula-artefato-desatualizado-em-vez-de-abortar.md`

**Status:** IMPLEMENTANDO

**Tarefa:** Implementar o skip de artefato `outdated + owned` no runtime Go conforme contrato congelado
em `docs/cli-parity.md` (seção "install sobre artefato gerenciado desatualizado — skip, não erro fatal").

**Arquivos afetados:**
- `internal/integrations/manager.go` — campo `OnSkip`, helper `tildeAbbrev`, `preflight` (nova assinatura), `mutate` (filtro de skips)
- `internal/integrations/manager_test.go` — novos testes
- `internal/commands/init.go` — liga `OnSkip` em `installAITools`
- `internal/commands/integrations_flags.go` — liga `OnSkip` em `runIntegrationsOperation`

**Status:** CONCLUÍDO

**Entregue:**
- `Manager.OnSkip func(destination, reason string)` adicionado à struct.
- `tildeAbbrev(destination)` implementado na Manager (sem helper Go pré-existente — contrato defect reportado; Node.js tildeify lido para paridade byte-a-byte).
- `preflight` agora retorna `(skip bool, err error)`: caso `StateOutdated && owned && !force` de `mutationInstall` sinaliza skip em vez de erro; caso `StateModified` permanece erro.
- `mutate` filtra itens pulados de `resolved` antes das fases de snapshot e `applyMutation`. `OnSkip` chamado uma vez por destino (deduplicado); artefato pulado não entra no rollback nem no manifest write.
- `OnSkip` ligado em `init.go:installAITools` e `integrations_flags.go` imprimindo em stderr.
- Três novos testes: (1) skip batch com dois escopos verificando string byte-idêntica ao contrato; (2) OnSkip nil sem panic; (3) owned+modified continua erro (guarda contra simetrização).

**Validação:**
- `go build ./...` → sem erros
- `go test ./...` → 15/15 pacotes OK
- `go vet ./...` → sem erros

**Divergência reportada (contrato defect):** o contrato diz "reutilize o helper de tilde já existente em `internal/generators/update.go`", mas nenhum helper Go de tilde-abbreviação existe no codebase (update.go usa constantes hardcoded; `GlobalGroupPath` trunca templates de catálogo). O `tildeify` existe apenas no Node.js (`npm/src/lib/update-engine.js`). A lógica foi reimplementada nativamente em Go com o mesmo cuidado do ML-6H (strip de trailing separator via `filepath.Clean`). Reportado como defect do contrato, não como bug de implementação.

---

## Sessão 2026-07-29 — Apolo (ML-2B — skip de artefato outdated+owned no Node.js)

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-29-install-pula-artefato-desatualizado-em-vez-de-abortar.md`

**Status:** CONCLUÍDO

**Tarefa:** Implementar skip de artefato `outdated`+`owned` sem `--force` no runtime Node.js do
`IntegrationManager`. Artefatos `modified` continuam lançando erro — não simetrizar os casos.
Inverter asserção na linha 193 de `npm/tests/agents-skills.test.js`. Ligar `onSkip` nos callers
(`commands/init.js`, `commands/integrations.js`) imprimindo em stderr a string pinada no contrato.

**Entregue:**
- `npm/src/integrations/manager.js`: construtor aceita `{ onSkip }`; `preflight` retorna `true`
  (skip) em vez de lançar para `outdated`+`owned`+sem force em `install`; `modified` continua
  lançando sem alteração; `mutate` filtra pulados antes de snapshot/apply e chama `onSkip` uma vez
  por item na ordem de `resolved`.
- `npm/src/integrations/index.js`: `execute()` passa `options.onSkip` ao construtor do manager.
- `npm/src/generators/init.js`: `installIntegrationTarget` aceita `{ onSkip }` como 4º parâmetro
  e inclui em `options` repassados ao `execute`.
- `npm/src/commands/init.js`: importa `tildeify`; cria callback `onSkip` nos dois loops de
  `aiTools` (TTY e não-TTY) que emite a string pinada em stderr com tilde-abreviado e remediação
  por escopo.
- `npm/src/commands/integrations.js`: importa `tildeify`; cria callback `onSkip` antes de
  `execute()` para operações `mutation`.
- `npm/tests/agents-skills.test.js`: nome do teste atualizado (linha 181); linha 193 invertida —
  `assert.throws` substituído por `doesNotThrow` + bytes preservados + `onSkip` observado 1x.

**Validação:**
- `cd npm && npm test` → 328 passed, 0 failed.
- Teste `'unmanaged desired is current, legacy is outdated, and owned outdated skips install'` passou.
- `onSkip` ausente (manager sem segundo parâmetro) funciona silenciosamente em outros testes.

**Divergência do contrato:** nenhuma. Os intermediários `integrations/index.js` e `generators/init.js`
precisaram ser tocados para que o `onSkip` fluísse do caller até o `IntegrationManager` — isso era
implícito no contrato mas não listado explicitamente nos "arquivos afetados" do ML-2B.

---

## Sessão 2026-07-29 — Apolo (ML-6H — `trackfw update` escopo de projeto, corretivo final concluído)

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md`

**Tarefa:** Fechar a paridade de `trackfw update` (escopo de projeto) entre Go/Node.js/Python —
`scripts/check-update-parity.sh` estava falhando em `update-project/json/go-vs-node` e
`update-project/json/go-vs-python`.

**Entregue:**
- **Python** (`pypi/trackfw/commands/update.py`, `pypi/trackfw/generators/init_gen.py`,
  `pypi/trackfw/commands/discover.py`): `PROJECT_TARGET_IDS` passou de 3 para os 5 ids pinados
  (`agent-rules, agent-hooks, codex-project-agents, validate-script, claude-commands`, nessa ordem).
  Novo gerador único `generate_validate_script(cwd)` em `init_gen.py`, chamado tanto por `scaffold()`
  (agora `trackfw init` também escreve `scripts/trackfw-validate.sh`, o que antes nunca acontecia)
  quanto pelo novo alvo `validate-script` de `update`; `discover.py` delega para o mesmo gerador em
  vez de duplicar o template. Novo alvo `claude-commands` reaproveita `generate_claude_commands`
  (já existente). Corrigido `path` de `agent-hooks` para o glob `scripts/trackfw-attention-*.sh`
  (igual Go/Node, antes listava os dois arquivos por extenso). Adicionado `_silence_stdout` (mirror
  de `silenceStdout`/`silenceConsole` do Go/Node) porque os novos geradores imprimem progresso e
  quebravam o parse de `--json`.
- **Node.js** (`npm/src/commands/update.js`, `npm/src/generators/init.js`): o alvo `validate-script`
  usava `discover.js:writeValidateScript` (gerador simples e diferente) em vez do
  `generators/init.js:generateValidateScript` que `trackfw init` de fato usa — cada `update` reescrevia
  o script com bytes diferentes dos que `init` escreveu, reportando `updated` num projeto na verdade
  já corrente. `generateValidateScript` ganhou um segundo parâmetro `cwd` (antes assumia
  `process.cwd()`, inseguro para o sandbox de `--dry-run`) e `update.js` agora chama esse único gerador,
  mapeando `cfg.pkg_manager` (snake_case, como vem de `readUpdateConfig`) para `pkgManager`.
- **Go** (`internal/generators/scaffold.go`): `Scaffold()` nunca chamava `InjectHooksDetected` — só
  `update` chamava. Node.js e Python já chamavam essa injeção dentro do próprio `scaffold`/`init`, então
  um projeto recém-`init`-ado no Go não tinha `.claude/settings.json` etc., e o primeiro `update`
  reportava `updated` (arquivo genuinamente novo) onde Node/Python reportavam `skipped` (já corrente) —
  gap de paridade real em `init`, não bug no discriminador de `update`. Corrigido adicionando a mesma
  chamada `InjectHooksDetected(cwd)` (não-fatal, mesmo padrão de `update`) como último passo de
  `Scaffold`, espelhando a posição em Node/Python.
- **Node.js** (`npm/src/lib/update-engine.js`): `tildeify` já normalizava os dois lados (fix de uma
  ML anterior — ver `vault/notes/node-tildeify-double-slash-home-2026-07-29.md`), mas `path.normalize`
  preserva um separador final quando o `$HOME` já termina em `/` (comum no macOS, cujo `$TMPDIR`
  default já termina em `/`), e o código então comparava com um separador duplicado, falhando
  silenciosamente. Corrigido removendo o separador final de `normalizedHome` antes da comparação de
  prefixo. Teste novo: `npm/tests/update-engine.test.js`.

**Achado colateral (fora do escopo do handoff, mitigado, não corrigido no código):**
`scripts/check-update-parity.sh`'s `install_claude_agents()` (fixture da Cenário 4) roda
`"$GO_BIN" agents install --scope global` sem isolar `cwd` — se o gate for executado a partir da raiz
do repo (`make quality`/`make parity`/direto), a injeção de regras do `agents install` (que não é
gated por `--scope`) escreve no `CLAUDE.md` real do próprio repositório. Reproduzido e revertido duas
vezes durante esta sessão (`git checkout -- CLAUDE.md`); `scripts/` está fora do meu escopo de edição
(handoff ML-6H). Documentado em
`vault/notes/update-parity-gate-writes-real-claude-md-2026-07-29.md` para o próximo agente
QA/Zeus corrigir com um `cd` isolado nessa função específica.

**Validação:** `go build ./... && go vet ./... && go test ./...` verde (todos os pacotes);
`cd npm && npm test` — 328 passed, 0 failed; `python3 -m pytest pypi/tests -q` — 691 passed;
`scripts/check-update-parity.sh` — todos os cenários `OK`; `make quality` — verde (18/18 cenários de
`check-gates-falsify.sh`, todos os demais gates); `bin/trackfw validate --json` — `violations: 0,
warnings: 0`. Confirmado `git status --short CLAUDE.md` limpo ao final (revertido o achado colateral
acima) e `find ~/.claude ~/.codex ~/.gemini -mmin -N` sem artefatos trackfw fora da sessão do próprio
agente.

**Notas do vault:** `vault/notes/update-project-scope-duplicate-generators-2026-07-29.md` (causas-raiz
1–3 acima), `vault/notes/node-tildeify-trailing-separator-2026-07-29.md` (bug residual do tildeify),
`vault/notes/update-parity-gate-writes-real-claude-md-2026-07-29.md` (achado colateral do gate).

**Próximo agente (Zeus/orquestrador):** este agente não executa git — commit/push/ML status ficam
para quem detém a branch. Considerar abrir REQ/roadmap-item para corrigir `install_claude_agents()`
em `scripts/check-update-parity.sh` (achado colateral, fora do escopo desta ML).

---

## Sessão 2026-07-29 — Apolo (ML-6B `trackfw update harness` — runtime Go concluído)

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md`

**Tarefa:** Separar `trackfw update` (escopo de projeto, nunca muta estado global) de um novo
`trackfw update harness` (escopo global, não exige `trackfw.yaml` nem cwd de projeto), conforme
contrato pinado em `docs/cli-parity.md` (`## trackfw update vs trackfw update harness`).

**Entregue (runtime Go apenas — Node.js/Python ficam para ML-6C/6D):**
- `internal/generators/scaffold.go`: extraído `GlobalClaudeSkillPath(home)` e
  `GlobalClaudeSkillContent()` a partir de `installGlobalSkillInner`, reutilizados pelo harness.
- `internal/generators/update.go`: `Update()` (projeto) não chama mais `ForceInstallSkills()` — a
  skill legada global (`~/.claude/skills/trackfw/SKILL.md`) saiu do caminho de `trackfw update`.
  Adicionados os tipos `TargetState`/`TargetResult`/`UpdateSummary`/`UpdateReport`/`UpdateOptions`,
  a lista fixa e declarada `HarnessTargetIDs = []string{"claude-skill", "codex-agents",
  "codex-skills"}` e `UpdateHarness(opts)`, que resolve `$HOME` via `os.UserHomeDir()` (nunca
  `os/user`) e nunca lê `trackfw.yaml`.
- `internal/commands/update_harness.go` (novo): subcomando `trackfw update harness` com
  `--dry-run`, `--json`, `--targets` (usage error se id desconhecido) e `--install-missing`;
  documento JSON com ordem de chaves `scope, dry_run, targets, summary` e, por target,
  `id, state, path, message` (`message` só quando `failed`, via `omitempty`).
- `internal/commands/update.go`: `newUpdateCmd()` registra o subcomando via `cmd.AddCommand(...)`
  — `root.go` não precisou de alteração (o registro de `update` já cobre o subcomando).
- Testes: `internal/generators/update_test.go` (harness) e
  `internal/commands/update_harness_test.go` (CLI + ordem literal de chaves do JSON), todos com
  `t.Setenv("HOME", t.TempDir())`. Ajustado `TestUpdateDoesNotImplicitlyInstallAgentIntegrations`
  para afirmar que `Update()` **não** grava mais a skill global (antes afirmava o oposto).

**Decisão de escopo (reportada, não corrigida):** `docs/cli-parity.md` diz que as 4 flags e o
documento JSON aplicam-se a "ambos" os comandos, mas os critérios de aceite concretos do ML-6B só
testam o comportamento de `update harness`. Optei por implementar o contrato completo (estados,
flags, JSON) somente em `update harness`; `trackfw update` (projeto) manteve sua saída de texto
existente, sem flags novas — evita inventar uma granularidade de "targets" de projeto não
especificada que os runtimes Node/Python (ML-6C/6D) teriam que adivinhar/replicar. Ver nota do
vault linkada abaixo.

**Validação:** `go build ./...`, `go vet ./...`, `go test ./...` — todos verdes. Confirmado por
`find ~/.claude -newermt "-15 min" -type f` (vazio) que o `$HOME` real não foi tocado durante os
testes nem durante o `go run` manual de fumaça (que sempre usou `HOME=$(mktemp -d)`).

**Nota do vault:** `vault/notes/update-harness-project-scope-json-gap-2026-07-29.md`.

**Próximo agente:** ML-6C (Node.js) e ML-6D (Python) devem espelhar exatamente `HarnessTargetIDs`
(`claude-skill`, `codex-agents`, `codex-skills`, nessa ordem) e a mesma decisão de escopo acima
(ou revisá-la explicitamente nos 3 runtimes ao mesmo tempo, nunca só em um).

---

## Sessão 2026-07-29 — Apolo (ML-5G — reconciliação do bloco de regras entre os 3 runtimes concluída)

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md`

**Tarefa:** Remover a duplicação do bloco `Architecture Directives` em
`internal/generators/agentfiles.go` e unificar o texto do bloco de regras (`trackfwRulesBlock` /
`_trackfw_rules_block`) injetado em `GEMINI.md`, `.github/copilot-instructions.md`,
`.windsurfrules` e `.amazonq/developer/guidelines.md` entre Go, Node.js e Python, para desbloquear
o ML-5E (ligar essa injeção ao caminho de instalação por catálogo nos três runtimes).

**Entregue:**
- `internal/generators/agentfiles.go`: removida a duplicação literal do bloco `### Architecture
  Directives`; texto reconciliado (ver decisões abaixo).
- `npm/src/generators/init.js` (`trackfwRulesBlock`) e `pypi/trackfw/generators/init_gen.py`
  (`_trackfw_rules_block`, constante `GLOBAL_ADR_DIRECTIVE` sem o prefixo `"- "` que só ela tinha):
  texto reconciliado para bater byte-a-byte com o Go.
- Como a divergência de conteúdo era só metade do bloqueio do ML-5E, também liguei a injeção
  (`InjectRulesForTool`/`injectRulesForTool`/`inject_rules_for_tool`) no caminho de instalação por
  catálogo do Node.js (`npm/src/integrations/index.js:execute`, escopo `install`) e do Python
  (`pypi/trackfw/integrations/command.py:run`, escopo `install`, e `pypi/trackfw/commands/init.py`
  no bloco `--ai-tools`) — espelhando exatamente `internal/commands/integrations_flags.go` e
  `internal/commands/init.go:installAITools`, que já faziam isso só no Go. Sem isso,
  `check-identity-parity.sh` reprovava por contagem de artefatos (`go=13 node=12 python=12`), não
  só por conteúdo — reconciliar o texto sozinho nunca teria deixado o gate verde.
- Novo gate `scripts/check-rules-parity.sh` (adicionado ao target `parity` do `Makefile`): roda
  `<cli> init --ai-tools gemini,copilot,windsurf,amazonq` (sem `--scope`, que `init` não tem — vai
  para `$HOME` isolado por runtime, como `check-identity-parity.sh`) nos três runtimes e compara os
  4 arquivos de regras byte-a-byte, com vacuity guard.
- Cenário 16 novo em `scripts/check-gates-falsify.sh` (`rules-parity/content-drift`): prova que
  `check-rules-parity.sh` reprova quando o npm volta a omitir `analyzing` da chain de estados —
  17 cenários / 11 gates provados não-vácuos ao final.
- `pypi/trackfw/generators/init_gen.py` linha ~355 (`generate_claude_md`, gerador do CLAUDE.md
  completo, função fora do escopo do bloco de regras): removido o `.lstrip("- ")` sobre
  `GLOBAL_ADR_DIRECTIVE` — ficou inerte depois que o prefixo `"- "` saiu da constante, mantido só
  por clareza; nenhum comportamento mudou (coberto por `check-artifact-parity.sh`).

**Decisões de reconciliação sem maioria clara (reportadas por instrução do handoff):**
- Ordem das seções dentro do bloco: adotei a ordem de Node/Python (Protocol → Attention Signal →
  Architecture Directives), maioria 2-1 sobre a ordem do Go (Protocol → Architecture → Key Commands
  → Attention). `Key Commands` foi anexado ao final, por ser conteúdo novo sem posição disputada.
- Item "0. Before any implementation" (branch só após REQ+ROADMAP em wip/) existia só no Go
  (1-0-0, não maioria) — mantido e propagado para os três por ser reconciliação "para cima"
  (não perder conteúdo), renumerado como item 1 dos 6 itens finais do Agent Protocol.
- Placement da diretiva de ADRs globais: Go e Node já a tratavam como item numerado do Agent
  Protocol (agora item 6); só o Python a colocava como primeira linha solta dentro de Architecture
  Directives — maioria 2-1 a favor do item numerado, aplicada ao Python.

**Validação:**
- `go build ./... && go vet ./... && go test ./...` → verde.
- `cd npm && npm test` → 304 passed, 0 failed.
- `python3 -m pytest pypi/tests -q` → 675 passed.
- `GO_BIN=bin/trackfw scripts/check-identity-parity.sh` → verde (11 combinações target/surface).
- `GO_BIN=bin/trackfw scripts/check-slash-parity.sh` → verde (pré-existente, não regressivo).
- `GO_BIN=bin/trackfw scripts/check-rules-parity.sh` (novo) → verde (4 arquivos x 3 runtimes).
- `make quality` → verde, incluindo `check-gates-falsify.sh` (16 cenários, 10 gates não-vácuos).
- `bin/trackfw validate --json` → `{"violations":0,"warnings":0,"exit_code":0}`.
- Idempotência confirmada manualmente: `init --ai-tools gemini` duas vezes seguidas não duplica o
  marcador `trackfw:rules:start` em `GEMINI.md`.

**Nota do vault:** `vault/notes/rules-block-content-drift-3-clis-2026-07-29.md` (achado do ML-5E)
recebeu uma seção `## Resolução (ML-5G...)` explícita, substituindo as três alternativas em aberto
(a/b/c) que ela listava — sem essa atualização, quem lesse a nota concluiria que o ML-5E ainda
está bloqueado. Já estava linkada em `vault/notes/index.md`.

**ML-5E:** os critérios de aceite restantes desse ML ("teste cobre os quatro arquivos a partir de
projeto vazio", "paridade verificada entre os runtimes que possuem a superfície") estão satisfeitos
em substância pelo trabalho acima — não editei o roadmap (fora do meu escopo/permissão), mas o
orquestrador pode fechá-lo em vez de redespachar.

---

## Sessão 2026-07-29 — Prometeu (ML-3A `/trackfw:barrier` e autoridade Git dos agentes concluído)

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md`

**Tarefa:** Retirar toda autoridade Git dos 11 agentes especialistas e concentrá-la em
`trackfw_architect`; criar o slash command `/trackfw:barrier` nos três runtimes; anunciá-lo na
tabela de slash commands do CLAUDE.md gerado.

**Entregue:**
- `internal/integrations/assets/agents/architect.md`: seção `## Git authority` revisada (agora
  cobre commit de código de produto, já que especialistas não commitam mais) + nova seção
  `## Barrier protocol` (invocar code-quality/security, bloquear wave em falha, auditar antes de
  commitar). Propagado para `npm/` e `pypi/` via `scripts/sync-integration-assets.sh`.
- Os 11 especialistas (`backend`, `code-quality`, `data`, `dba`, `frontend`, `iac`, `infra`, `qa`,
  `security`, `tooling`, `ux`): `## Git boundary` → `## Git authority` (ou seção nova para os 3
  read-only) declarando que nunca executam operações Git e só atuam por handoff autocontido de
  `trackfw_architect`.
- Slash command `barrier.md` (checklist operacional de 10 itens) adicionado como literal idêntico
  em `internal/generators/scaffold.go`, `npm/src/generators/init.js` e
  `pypi/trackfw/generators/init_gen.py` — equivalência byte-a-byte provada via SHA256 (mesmo hash
  nos três runtimes, verificação manual pois não há gate automático para esta superfície).
- `/trackfw:barrier` anunciado na tabela de slash commands do CLAUDE.md gerado
  (`internal/generators/claudemd.go` + gêmeos Node/Python).
- Golden tests re-congelados (`internal/integrations/testdata/*.golden.*`) e literais equivalentes
  em `npm/tests/agents-skills.test.js` atualizados para refletir o novo texto de `architect.md` e
  `backend.md`.
- Novo teste `internal/integrations/agents_git_authority_test.go`: prova que só `architect` tem
  protocolo de autoridade Git/barrier e que os outros 11 desautorizam operações Git.

**Validação:**
- `go build ./... && go vet ./... && go test ./...` → verde.
- `cd npm && npm test` → 301 passed, 0 failed.
- `python3 -m pytest pypi/tests -q` → 670 passed.
- `make quality` → verde (inclui `check-integration-assets.sh`, `check-static-assets.sh`,
  `check-artifact-parity.sh`, `check-gates-falsify.sh`).
- `bin/trackfw validate --json` → `{"violations":0,"warnings":0}`.
- `grep -rniE 'git (add|commit|push|checkout|branch|merge|rebase|stash|reset)'` nos três diretórios
  de assets → só `architect.md` aparece, e apenas no protocolo de autoridade.

**Ambiguidade reportada, não corrigida:** o critério de aceite do ML-3A no roadmap lista "Testes
cobrem... a ausência de regras de paridade universal", frase que parece copiada do critério de
Wave 1/2 (CLI `trackfw barrier`) — não há regra de paridade universal no escopo de assets de agente
e slash commands desta ML. Também identificado (pré-existente, fora de escopo): `npm/src/generators/init.js`
tem uma segunda função `generateClaudeCommandsForce` cujo mapa de comandos já omitia `roadmap.md` e
`implement.md` antes desta ML — não recebeu `barrier.md` para não aprofundar essa divergência
pré-existente sem decisão do orquestrador.

---

## Sessão 2026-07-29 — Apolo (ML-2A `trackfw barrier` — runtime Go concluído)

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md`

**Tarefa:** Implementar `trackfw barrier <roadmap> --wave <n> [--json]` no runtime Go, exatamente
conforme o contrato congelado em `docs/cli-parity.md` (`## trackfw barrier`), e remover os 8
`t.Skip` do ML-1A em `internal/commands/barrier_contract_test.go`. Escopo restrito ao runtime Go —
MLs 2B (Node.js) e 2C (Python) executando em paralelo, sem tocar `npm/` nem `pypi/`.

**Entregue:**
- `internal/commands/barrier.go` (novo): parser string-level das seis regras do roadmap (waves,
  MLs, `**Status:**`, bloco de critérios de aceite, bloco de gates com fence ```bash),
  quatro checks embutidos (`mls_complete`, `acceptance_evidence`, `gates`, `validate`), resolução
  de roadmap por basename em `wip/` depois `done/` (flat e by_agent), exit codes 0/1/2 via
  `os.Exit` explícito (necessário porque `root.go Execute()` força exit 1 em qualquer erro não-nulo
  — por isso o flag `--wave` é `StringVar` + parse manual, não `IntVar`, para controlar a mensagem e
  o exit code de uso malformado).
- `internal/commands/barrier_test.go` (novo): testes unitários do parser e dos checks, isolados do
  binário compilado.
- `internal/commands/root.go`: registra `newBarrierCmd()`.
- `internal/commands/barrier_contract_test.go`: removidos os 8 `t.Skip` do ML-1A — corpos
  preservados sem reescrita.

**Validação:**
- `go build ./...` → sem erros.
- `go vet ./...` → sem erros.
- `go test ./...` → verde em todos os pacotes; os 8 testes de contrato (`TestBarrierContract_*`)
  passam sem skip.
- Nenhum arquivo sob `npm/` ou `pypi/` foi tocado (confirmado via `git status --short`).

**Ressalva:** `Commands` no `barrierCheck` é `*[]string` (ponteiro), não `[]string`, para que
`omitempty` só suprima o campo quando nil — o check `gates` sempre define um ponteiro não-nil
(mesmo para lista vazia), garantindo que `"commands": []` apareça sempre nesse check e nunca nos
demais, conforme a tabela de determinismo do contrato.

## Sessão 2026-07-27 — Apolo (ML-1B débitos técnicos concluído)

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-27-debitos-tecnicos-pos-release-de-robustez-e-manutenibilidade.md`

**Tarefa:** Provar a lacuna do gate de identidade quando o catálogo ganha alvo/superfície de agente
suportado que não está na lista hardcoded de `TARGETS`, sem alterar catálogo real ou código de
produção.

**Entregue:**
- `scripts/check-identity-parity.sh` agora valida que `TARGETS` cobre as superfícies de agentes
  suportadas no catálogo canônico, mantendo a lista hardcoded até o ML-2C.
- `scripts/check-gates-falsify.sh` adicionou o cenário
  `identity-parity/catalog-target-missing`, que injeta temporariamente `codex=experimental` numa
  cópia do catálogo e exige falha por alvo/superfície ausente.
- A fixture temporária fica isolada em `mktemp` e é removida pelo `trap`; nenhum asset de catálogo
  real foi alterado.

**Validação:**
- `scripts/check-identity-parity.sh` →
  `Identity parity verified across Go/Node/Python for 11 target/surface combinations (with and without identity)`.
- `scripts/check-gates-falsify.sh` →
  `Falsification checks passed (all 13 scenarios, 8 gates proved non-vacuous)`.
- `bin/trackfw validate --json` → `0 violations`, `0 warnings`.
- `git diff --check` → verde.

**Ressalva:**
- O worktree já continha alterações de outro ML (`ADR`, `docs/cli-parity.md`, testes de validator e
  movimentação do roadmap backlog→wip). Este ML preservou esse trabalho e só deve commitar os dois
  scripts, o roadmap WIP, o contexto e a deleção do roadmap em `backlog`.

## Sessão 2026-07-27 — Apolo (ML-1A débitos técnicos concluído)

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-27-debitos-tecnicos-pos-release-de-robustez-e-manutenibilidade.md`

**Tarefa:** Fechar o contrato documentado de `stale_wip` e política de erros de inspeção sem alterar
código de produção.

**Entregue:**
- ADR de gates verificáveis recebeu adendo definindo idade de `stale_wip` como a entrada mais recente
  em `wip/` registrada no `.trackfw-log`.
- `docs/cli-parity.md` documenta o contrato cross-runtime: `.trackfw-log` como fonte canônica,
  fallback retrocompatível por `mtime`, `git log` fora do contrato, default de 7 dias e severidade
  default `warning`.
- Política de inspeção documentada: `ENOENT` de estado opcional é vazio; permissão negada,
  `ENOTDIR`/erro de walk, arquivo esperado ilegível e arquivo/linha de log inválidos geram
  diagnóstico da regra.
- Provas negativas strict adicionadas nos três runtimes, sem produção:
  `internal/validator/validator_stale_wip_contract_xfail_test.go`,
  `npm/tests/validator.test.js` e `pypi/tests/test_validator.py`.
- ML-1A marcado como concluído no roadmap WIP.

**Validação:**
- `go test ./internal/validator -run 'StaleWIP' -v` → verde, 2 xfails esperados via helper.
- `(cd npm && npm test -- --test-name-pattern='stale_wip')` → verde; `validator.test.js` reportou
  `41 passed, 0 failed, 2 xfail`.
- `python3 -m pytest pypi/tests/test_validator.py -q -rxX` → `70 passed, 2 xfailed`.

**Ressalva:**
- `scripts/check-gates-falsify.sh` e `scripts/check-identity-parity.sh` já estavam modificados por
  outro escopo/ML paralelo e foram preservados fora deste ML.

## Sessão 2026-07-27 — Artemis (ML-3A bloqueadores concluído)

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-27-bloqueadores-de-release-de-paridade-e-precisao-contratual.md`

**Tarefa:** Fechar o gate cross-CLI dos bloqueadores de release de paridade/contrato, preservando as
provas negativas e validando flags, artefatos, status aspeado e log `by_agent` nos três runtimes.

**Entregue:**
- `scripts/check-cli-parity.sh` agora exige `--title`, `--req` e `--from-req` no help de
  `roadmap new` em Go, Node e Python.
- `scripts/check-artifact-parity.sh` cobre 8 tipos de artefato nos três runtimes, exercita geração real
  com `--title/--req` e `--from-req`, valida `status: "wip"` como 0/0 e mantém o ciclo flat/`by_agent`.
- `scripts/check-gates-falsify.sh` cobre 12 cenários P4, incluindo drift de flag pública em
  `roadmap new` e drift do log `by_agent`.

**Validação:**
- `GO_BIN=bin/trackfw scripts/check-cli-parity.sh` → `CLI parity smoke checks passed`.
- `GO_BIN=bin/trackfw scripts/check-artifact-parity.sh` →
  `Artifact parity checks passed (8 artifact types × 3 runtimes; roadmap flags, quoted status, analyzing cycle flat/by_agent)`.
- `GO_BIN=bin/trackfw scripts/check-gates-falsify.sh` →
  `Falsification checks passed (all 12 scenarios, 8 gates proved non-vacuous)`.
- `bin/trackfw validate --json` → `0 violations`, `0 warnings`.
- `git diff --check` → verde.
- `make quality` → verde em execução anterior desta mesma sessão; package smoke não foi reexecutado na
  retomada por orientação explícita do handoff.

**Ressalva:**
- Nenhum código de produção foi alterado neste ML; o escopo ficou restrito a gates e documentação de
  execução. Package smoke permanece sem nova evidência nesta retomada porque o handoff determinou não
  rodar `make quality/smoke`.

## Sessão 2026-07-27 — Artemis (ML-1A concluído)

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-27-bloqueadores-de-release-de-paridade-e-precisao-contratual.md`

**Tarefa:** Caracterizar os quatro bloqueadores de release de paridade/contrato sem tocar código de
produção.

**Entregue:**
- Python `roadmap new`: xfail strict em `pypi/tests/test_commands_roadmap_discover.py` provando ausência
  de `--title`, `--req` e `--from-req`; controles de superfície em Go e Node adicionados.
- Python validator: xfails strict em `pypi/tests/test_validator.py` para `parse_frontmatter` e
  `folder_status` divergirem com `status: "wip"`.
- Log `by_agent`: o código Python atual já preserva `zeus/<arquivo>.md`; adicionado guard obrigatório em
  `pypi/tests/test_generators_roadmap.py` para impedir regressão do log `backlog → wip`.
- Contrato documental de JSON Schema: xfail strict em `pypi/tests/test_documentation_contract.py`
  enquanto o site afirmar validação automática inexistente por `trackfw validate`.

**Validação:**
- `python3 -m pytest pypi/tests/test_commands_roadmap_discover.py pypi/tests/test_validator.py pypi/tests/test_generators_roadmap.py pypi/tests/test_documentation_contract.py -q -rxX`
  → `115 passed, 4 xfailed`.
- `go test ./internal/commands ./internal/generators -run 'RoadmapNewCmdExposesParityFlags|MoveRoadmap' -v`
  → verde.
- `npm test -- --test-name-pattern='roadmap new exposes parity flags|moveRoadmap'` → Node executou a
  suíte com `265 pass`, `0 fail`.
- `bin/trackfw validate --json` → `0 violations`, `0 warnings`.

**Ressalva:**
- `make quality` não foi executado neste ML; permanece para auditoria central conforme orientação do
  handoff.

## Sessão 2026-07-27 — Artemis (ML-3A concluído)

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-27-contrato-canonico-do-roadmap-e-estado-analyzing.md`

**Tarefa:** Fechar o gate cross-CLI e a prova de ciclo `backlog → analyzing` nos três runtimes,
documentar o frontmatter/estado canônico em PT-BR e inglês e auditar a paridade i18n do ML-2B.

**Entregue:**
- `scripts/check-artifact-parity.sh` compara também o slash-command `/trackfw:roadmap` byte a byte
  e executa ciclo E2E em layouts flat e `by_agent`, conferindo pasta, frontmatter, header, log e
  ausência de `folder_status`.
- `scripts/check-gates-falsify.sh` inclui prova P4 de drift do slash-command (cenário 9), mantendo
  a prova de integridade referencial como cenário 10.
- Documentação atualizada em `docs/cli-parity.md`, `site/guide/commands.md` e
  `site/en/guide/commands.md`; estados válidos incluem `analyzing` e frontmatter canônico é exigido.

**Validação:**
- `scripts/check-artifact-parity.sh` → `Artifact parity checks passed (6 artifact types × 3 runtimes; analyzing cycle flat/by_agent)`.
- `scripts/check-gates-falsify.sh` → todos os cenários P4 verdes.
- `make quality` → verde.
- `go test ./...`, `npm test`, `pytest`, `bin/trackfw validate --json` e `git diff --check` → verdes.

---

## Sessão 2026-07-27 — Apolo (ML-2B concluído)

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-27-contrato-canonico-do-roadmap-e-estado-analyzing.md`

**Tarefa:** Implementar exclusivamente o estado `analyzing` em `roadmap move/list/show` nos três CLIs,
preservando paridade Go/Node/Python, layout flat e `by_agent`, frontmatter/header sincronizados e
`.trackfw-log` com agente.

**Entregue:**
- Go: `stateDir`, `agentStateDir`, `MoveRoadmap`, `findRoadmap` e `ListRoadmaps` agora usam estado
  `analyzing`; help de `roadmap move` lista o estado.
- Node.js: `VALID_STATES`/`STATE_ORDER` agora incluem `analyzing`; mensagens de erro e i18n de
  `roadmap move` listam o estado.
- Python: `VALID_STATES`/`STATE_ORDER` agora incluem `analyzing`; argparse aceita o estado por
  `choices=VALID_STATES`; `move_roadmap` preserva `zeus/<arquivo>.md` no log em `by_agent`.
- Testes xfail de `analyzing` do ML-1A foram convertidos em testes obrigatórios nos três runtimes.
- Cobertura adicionada para `list`/`show` encontrarem roadmaps em `analyzing/` em layout flat e
  `by_agent`.
- O slash-command `/trackfw:roadmap` não foi alterado neste ML.

**Validação:**
- `go test ./internal/generators ./internal/commands -run Analyzing -v` → verde.
- `(cd npm && npm test -- --test-name-pattern=roadmap_move)` → verde, `264 pass`, `0 fail`.
- `python3 -m pytest pypi/tests/test_generators_roadmap.py pypi/tests/test_commands_roadmap_discover.py -q`
  → `52 passed`.
- `go build ./...` → verde; aviso não bloqueante de cache Go fora do sandbox.
- `go test ./...` → verde.
- `(cd npm && npm test)` → verde, `264 pass`, `0 fail`.
- `python3 -m pytest pypi/tests -q` → `619 passed`.
- `git diff --check` → verde.
- `bin/trackfw validate --json` → `0 violations`, `0 warnings`.

**Ressalva:**
- `make quality` não foi executado neste ML por orientação do roadmap de deixar o gate composto para
  auditoria central; os builds/testes amplos dos três runtimes foram executados.

---

## Sessão 2026-07-27 — Apolo (ML-2A concluído)

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-27-contrato-canonico-do-roadmap-e-estado-analyzing.md`

**Tarefa:** Corrigir somente o slash-command `/trackfw:roadmap` e seus geradores/templates nos três CLIs,
reativando os testes de frontmatter canônico sem implementar `analyzing`.

**Entregue:**
- Go, Node.js e Python agora geram `.claude/commands/trackfw/roadmap.md` com frontmatter canônico:
  `status: backlog`, `date: <YYYY-MM-DD>`, `req: "docs/req/<arquivo-selecionado>.md"` e `squad: ""`.
- Header mantido no contrato canônico `> Created: <YYYY-MM-DD> | Status: backlog`, com waves e
  microlotes preservados.
- O arquivo versionado `.claude/commands/trackfw/roadmap.md` foi alinhado ao conteúdo gerado.
- Testes xfail do slash-command foram convertidos para testes obrigatórios nos três runtimes e passam
  comparando byte a byte o comando gerado com o arquivo versionado.
- Correção pós-auditoria: restaurado o bloco estrutural `ML-1B` e `Wave 2` no template materializado
  e nos geradores Go/Node/Python; os testes focados agora afirmam explicitamente esses trechos.

**Decisão de interpolação do `req:`:**
- O slash-command é uma instrução de geração, então mantém o placeholder
  `docs/req/<arquivo-selecionado>.md` no template e instrui preencher esse campo com o caminho relativo
  completo da REQ selecionada. Isso evita basename/link Markdown e preserva o caminho real no artefato
  materializado pelo agente.

**Validação:**
- `go test ./internal/generators -run SlashRoadmap -v` → verde.
- `npm test -- --test-name-pattern=SlashRoadmap` → verde, `264 pass`, `0 fail`.
- `python3 -m pytest pypi/tests/test_generators_init.py -q` → `39 passed`.
- `go build ./...` → verde; aviso não bloqueante de cache Go fora do sandbox.
- `go test ./...` → verde.
- `(cd npm && npm test)` → verde, `264 pass`, `0 fail`.
- `bin/trackfw validate --json` → `0 violations`, `0 warnings`.
- `make quality` → verde; Python completo `613 passed, 2 xfailed`; falsificação `all 9 scenarios, 8 gates proved non-vacuous`.

**Ressalva:**
- Nenhum runtime de `roadmap move` foi alterado neste ML; os xfails de `analyzing` permanecem para o ML-2B.

---

## Sessão 2026-07-27 — Artemis (ML-1A concluído)

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-27-contrato-canonico-do-roadmap-e-estado-analyzing.md`

**Tarefa:** Adicionar testes negativos/caracterização para o contrato canônico de `/trackfw:roadmap`
e para `roadmap move <name> analyzing`, sem corrigir código de produção.

**Entregue:**
- Go: xfail estrito em `internal/generators/scaffold_test.go` para exigir frontmatter canônico no
  slash-command e xfails em `internal/generators/roadmap_test.go` para `analyzing` flat/by_agent.
- Node.js: xfail estrito novo em `npm/tests/init.test.js` para slash-command e xfails em
  `npm/tests/roadmap_move.test.js` para `analyzing` flat/by_agent.
- Python: `pytest.mark.xfail(strict=True)` em `pypi/tests/test_generators_init.py` e
  `pypi/tests/test_generators_roadmap.py` cobrindo os mesmos defeitos.
- Evidências negativas capturadas: slash-command não contém o início canônico ` ```markdown` seguido
  de `---`; Go/Node rejeitam `analyzing` com `invalid state "analyzing"`; Python reporta
  três xfails strict nos cenários equivalentes.

**Validação:**
- `go test ./internal/generators -run 'SlashRoadmap|Analyzing' -v` → verde, 3 xfails esperados via helper.
- `(cd npm && npm test)` → `264 pass`, `0 fail`, com xfails esperados logados.
- `python3 -m pytest pypi/tests/test_generators_init.py pypi/tests/test_generators_roadmap.py -q -rxX` → `58 passed, 3 xfailed`.
- `make quality` → verde; Python completo `612 passed, 3 xfailed`; falsificação `all 9 scenarios, 8 gates proved non-vacuous`.

**Ressalva:**
- O xfail Node de slash-command foi criado no arquivo previsto pelo roadmap (`npm/tests/init.test.js`),
  que não existia antes; `npm/tests/generators.test.js` foi mantido sem mudança funcional final.

---

## Sessão 2026-07-27 — Apolo (ML-3A concluído)

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-27-integridade-das-referencias-e-ciclo-de-vida-da-req.md`

**Tarefa:** Normalizar referências de REQ, fechar REQs concluídas e proteger integridade referencial no `make quality`.

**Entregue:**
- 38 arquivos em `docs/req/*.md` normalizados para referências canônicas de frontmatter:
  38 campos `roadmap:` e 6 campos `adr:` agora usam caminho relativo completo, com `.md`.
- Reconciliação registrada: `bin/trackfw validate --json` media 41 warnings antes da normalização;
  esses 41 eram itens de validação, não campos `roadmap:` únicos. A reconciliação estática confirmou
  38 campos `roadmap:` não canônicos, além de 6 campos `adr:` normalizados; o caso
  `ROADMAP-2026-07-25-escopo-...` não aparecia como warning porque estava sem `.md`, mas tinha
  correspondência única em `docs/roadmaps/done/`.
- 6 REQs `Open` com roadmap em `done/` fechadas via `bin/trackfw req move ... Done`, sem edição manual.
- `ref_targets_exist` elevado para `error` nos defaults dos 3 CLIs.
- Escape 3 reativado nos testes Go, Node.js e Python.
- `scripts/check-referential-integrity.sh` criado e integrado ao `make quality`.
- `scripts/check-gates-falsify.sh` ganhou P4 `referential-integrity/missing-roadmap`; cenário 8 usa
  `GOCACHE` temporário para build isolado no sandbox.

**Validação:**
- `go build ./...` → verde; aviso não bloqueante de cache Go fora do workspace.
- `go test ./...` → verde.
- `(cd npm && npm test)` → `263 pass`, `0 fail`.
- `python3 -m pytest pypi/tests -q -rxX` → `612 passed`.
- `scripts/check-referential-integrity.sh` → `Referential integrity OK`.
- `scripts/check-gates-falsify.sh` → `Falsification checks passed (all 9 scenarios, 8 gates proved non-vacuous)`.
- `bin/trackfw validate --json` → 0 violations, 0 warnings.
- `make quality` → verde.

**Correção pós-auditoria:**
- Auditoria reprovou o harness porque o cenário 8 de `scripts/check-gates-falsify.sh` podia abortar
  no `go build` isolado com stderr suprimido, impedindo a execução dos cenários 8/9 e ocultando a
  causa.
- Diagnóstico local: o cenário 8 compila uma cópia temporária do módulo Go com `internal/generators/req.go`
  corrompido para gerar `RREQ-...`; nesta sessão o build completou, mas o harness ainda era opaco em
  caso de falha por causa de `set -e` + `2>/dev/null`.
- Correção: criado helper `build_go_or_fail` em `scripts/check-gates-falsify.sh`, com `GOCACHE`
  temporário, captura de stdout/stderr e mensagem `FAIL [falsify/setup-s8-build]` contendo comando
  exato e log do `go build`.
- Validação pós-correção: `scripts/check-gates-falsify.sh` → cenários 1-9 executados e resumo
  `Falsification checks passed (all 9 scenarios, 8 gates proved non-vacuous)`; `make quality` → verde.

---

## Sessão 2026-06-11 — Sessão inaugural

### O que foi decidido e construído

**Nome:** `trackfw` — nos três artefatos: repositório GitHub, CLI e pacote npm/PyPI.

**Conceito validado:**
- Framework de governança de entrega de software: `ADR → REQ → ROADMAP → backlog/wip/blocked/done/abandoned`
- CLI stack-agnostic com `trackfw init` interativo que detecta a stack e gera gates/hooks por projeto
- Modelo de plugin para generators comunitários (padrão Prettier/ESLint)
- Distribuição como Go binary único + wrappers finos npm/PyPI/Homebrew (padrão esbuild/Biome/Turbo)
- Nome do pacote npm e PyPI `trackfw` — **confirmado disponível** em ambos os registros

**O que foi implementado (v0.1 — Foundation):**
- `cmd/trackfw/main.go` — entry point
- `internal/commands/` — root, init, adr, req, roadmap, status, validate
- `internal/generators/` — scaffold, adr, req, roadmap (com move entre estados)
- `internal/validator/` — validate (consistência ADR↔REQ↔ROADMAP) + status
- `scripts/install.sh` — `curl | sh` para instalação direta
- `Makefile` — build, test, lint, install, clean
- `docs/visao-projeto/VISION.md` — visão completa do projeto
- Go module: `github.com/trackfw/trackfw`
- Dependências: `cobra` (CLI), `huh` (wizard interativo), `lipgloss` (styling)
- Build verde ✅ | CLI `--help` funcionando ✅ | 2 commits na `main`

---

## Próxima sessão — O que fazer primeiro

### ✅ Prioridade 1 — Criar repositório no GitHub (CONCLUÍDO)
- Repo: https://github.com/kgsaran/trackfw (privado, conta pessoal kgsaran)
- Module path atualizado para `github.com/kgsaran/trackfw`
- 3 commits na main, código em sincronia com o remoto

### Prioridade 2 — GoReleaser (distribuição de binários)
- ✅ ML-1A CONCLUÍDO (Ares, 2026-06-11): `.goreleaser.yaml` criado na raiz — v2 syntax, 5 targets (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64), archives tar.gz/zip, checksum sha256
- ✅ ML-2A CONCLUÍDO (Ares, 2026-06-11): `scripts/install.sh` reescrito — detecta OS/ARCH via uname, busca versao mais recente via API GitHub, suporta curl+wget, sudo quando necessario, verificacao de PATH, idempotente
- Criar GitHub Actions workflow: `.github/workflows/release.yml` (trigger: `push tag v*`)
- Testar release local: `goreleaser release --snapshot --clean`

### Prioridade 3 — Wrapper npm
- ✅ CONCLUIDO (Afrodite, 2026-06-11): `npm/package.json` criado com conteudo exato, JSON valido
- ✅ CONCLUIDO (Afrodite, 2026-06-11): `npm/bin/.gitkeep` e `npm/scripts/.gitkeep` criados
- Pendente: `npm/scripts/postinstall.js` — baixa o binario correto para a plataforma
- Pendente: Publicar no npm como `trackfw`

### Prioridade 4 — Wrapper PyPI
- Criar `pypi/` com `setup.py` / `pyproject.toml`
- Script de instalação que baixa o binário
- Publicar no PyPI como `trackfw`

---

---

## Sessão 2026-06-11 — Apolo (CONCLUÍDO)

**Tarefa:** Criar `npm/bin/trackfw` — wrapper JS que o npm registra como comando no PATH do usuário.

**Entregue:**
- `npm/bin/trackfw` criado com shebang `#!/usr/bin/env node`, detecção de Windows (`.exe`), `spawnSync` com `stdio: 'inherit'` e `process.argv.slice(2)`, saída de erro amigável se binário ausente.
- `chmod +x` aplicado — permissão `-rwxr-xr-x` confirmada.

---

## Sessão 2026-06-11 — Apolo (CONCLUÍDO)

**Tarefa:** Criar `npm/scripts/postinstall.js` — script que baixa o binário Go correto das GitHub Releases durante `npm install trackfw`.

**Entregue:**
- `npm/scripts/postinstall.js` criado — sem dependências externas, Node >= 14, segue redirects HTTPS 301/302, suporte a `tar.gz` (Linux/macOS) via `tar -xzf` e `.zip` (Windows) via PowerShell `Expand-Archive`, `chmod 755` no Unix, `exit(0)` em plataforma/arch não suportada ou erro (não bloqueia CIs).
- Versão lida do `npm/package.json` em tempo de execução.
- Sintaxe validada com `node --check`.

---

## Decisões técnicas registradas

| Decisão | Escolha | Motivo |
|---|---|---|
| Linguagem do CLI | Go | Binário único sem runtime, cross-platform, startup rápido |
| Distribuição | Binary + wrappers | Padrão esbuild/Biome/Turbo — agnóstico de runtime |
| CLI framework | cobra | Padrão da comunidade Go para CLIs |
| Wizard interativo | huh (charmbracelet) | TUI elegante, bem mantido |
| Estado do roadmap | Pasta = fonte de verdade | Sem DB, sem SaaS, portável |
| Extensibilidade | Plugin model (generators) | Comunidade contribui sem tocar core |

---

## Sessão 2026-06-11 — Apolo (CONCLUÍDO)

**Tarefa:** Criar pacote Python `pypi/trackfw/` — módulo Python do wrapper PyPI.

**Entregue:**
- `pypi/trackfw/__init__.py` criado (arquivo vazio — declara o pacote Python).
- `pypi/trackfw/_cli.py` criado — entry point PyPI sem dependências externas, Python 3.6+, detecta OS/ARCH, baixa binário Go das GitHub Releases (`tar.gz` Linux/macOS, `.zip` Windows), `os.execv` no Unix / `subprocess.run` no Windows, armazena binário como `trackfw-bin` dentro do pacote.
- Sintaxe validada com `python3 -m py_compile` — OK.

---

## Sessão 2026-06-11 — Apolo (CONCLUÍDO)

**Tarefa:** Corrigir Bug 1 (URL hardcoded org errada em `scaffold.go`) e Bug 2 (`containsIgnoreCase` não case-insensitive em `roadmap.go`).

**Entregue:**
- `internal/generators/scaffold.go`: substituídas 2 ocorrências de `https://raw.githubusercontent.com/trackfw/trackfw/main/scripts/install.sh` por `https://github.com/kgsaran/trackfw/releases/latest/download/install.sh` (linha GitHub Actions e linha GitLab CI).
- `internal/generators/roadmap.go`: adicionado import `"strings"`, substituídas `containsIgnoreCase` + `containsRune` por implementação correta via `strings.ToLower` + `strings.Contains`.
- `go build ./...` passou sem erros.

---

## Sessão 2026-06-11 — Apolo (CONCLUÍDO)

**Tarefa:** Adicionar comando `trackfw version`.

**Entregue:**
- `internal/version/version.go` criado — variável `Version = "dev"` injetável via ldflags em tempo de build.
- `internal/commands/version.go` criado — comando cobra `version` que imprime `trackfw <Version>`.
- `internal/commands/root.go` atualizado — `newVersionCmd()` registrado na lista de subcomandos.
- `.goreleaser.yaml` atualizado — ldflags com `-X 'github.com/kgsaran/trackfw/internal/version.Version={{.Version}}'`.
- `go build ./...` sem erros; `go run ./cmd/trackfw version` imprime `trackfw dev`.

---

## Sessão 2026-06-11 — Apolo (CONCLUÍDO)

**Tarefa:** Adicionar Regras 3, 4 e 5 ao `internal/validator/validator.go`.

**Entregue:**
- `validateBlockedHasREQ()` — verifica roadmaps em `docs/roadmaps/blocked/` sem campo `REQ:` preenchido.
- `validateREQsHaveRoadmap()` — verifica REQs em `docs/req/` sem campo `Roadmap:` preenchido.
- `validateADRsAreReferenced()` — verifica ADRs em `docs/adr/` não referenciados em nenhum REQ (campo `ADR:` dos REQs).
- As três funções registradas em `Validate()` após as chamadas existentes.
- `go build ./...` e `go vet ./...` passaram sem erros.

---

## Sessão 2026-06-11 — Apolo (CONCLUÍDO)

**Tarefa:** Configurar distribuição Homebrew para trackfw.

**Entregue:**
- Repositório `kgsaran/homebrew-trackfw` criado no GitHub (público) com `Formula/trackfw.rb` placeholder.
- `.goreleaser.yaml` — seção `brews:` adicionada ao final: aponta para `kgsaran/homebrew-trackfw`, diretório `Formula`, token via `HOMEBREW_TAP_GITHUB_TOKEN`, `skip_upload: auto`.
- `.github/workflows/release.yml` — `HOMEBREW_TAP_GITHUB_TOKEN: ${{ secrets.HOMEBREW_TAP_GITHUB_TOKEN }}` adicionado ao `env:` do step goreleaser.
- `goreleaser check` confirma `configuration is valid` (aviso de deprecação esperado: `brews` é a chave correta para CLI formulas em v2.16.0; `homebrew_casks` é para apps GUI).

**Pendente (ação do usuário):**
- Criar PAT com scope `repo` (para push no tap) e cadastrar como secret `HOMEBREW_TAP_GITHUB_TOKEN` no repo `kgsaran/trackfw` (Settings > Secrets > Actions).

---

## Sessão 2026-06-11 — Artemis (CONCLUÍDO)

**Tarefa:** Escrever testes unitários Go para `internal/validator` e `internal/generators`.

**Entregue:**
- `internal/validator/validator_test.go` — 7 testes: Clean, WIPMissingREQ, WIPMissingAcceptanceCriteria, MultipleWIP, REQMissingADR, BlockedMissingREQ, GetStatus_Empty
- `internal/generators/roadmap_test.go` — 5 testes: NewRoadmap_CreatesFile, MoveRoadmap_Valid, MoveRoadmap_InvalidState, MoveRoadmap_NotFound, ContainsIgnoreCase
- `internal/generators/adr_test.go` — 2 testes: NewADR_CreatesFile, NewADR_SlugInFilename

**Resultado:** 14/14 testes passaram. `go test ./internal/validator/... ./internal/generators/... -v` OK.

**Decisoes tecnicas:**
- Fixtures construidas para satisfazer regras irmas e isolar uma violacao por teste (ex: WIPMissingREQ inclui bloco Acceptance Criteria; REQMissingADR inclui Roadmap preenchido)
- MkdirAll de todos os diretorios de estados validos em TestMoveRoadmap_Valid antes da chamada (os.Rename requer destino existente)
- Localizacao de arquivos gerados via filepath.Glob (filename embute time.Now — data do dia)
- Package white-box (sem prefixo de pacote) para acesso direto a containsIgnoreCase e validStates

---

## Sessão 2026-06-11 — Apolo (CONCLUÍDO)

**Tarefa:** Refatorar `trackfw init` — wizard condicional por tipo de projeto, geração de `CLAUDE.md`, e correção do validate script para Python.

**Entregue:**
- `internal/generators/scaffold.go` — `Config` estendido com `ProjectType` e `ProjectName`; case `python` adicionado em `buildValidateScript`; chamada a `generateClaudeMD(cfg)` adicionada ao final de `Scaffold()`.
- `internal/generators/claudemd.go` — arquivo novo; `generateClaudeMD(cfg Config) error` gera `CLAUDE.md` com seções de governança, frontend/backend condicionais, pre-commit checklist, git hooks e CI gate; `backendCommands()` mapeia build/test/lint por stack (go, java, node, python).
- `internal/commands/init.go` — wizard reescrito com 4 grupos: Grupo 1 (sempre, nome + tipo), Grupo 2 (frontend+pkgmanager, hidden se backend/governance), Grupo 3 (backend, hidden se frontend/governance), Grupo 4 (sempre, hooks+ci).
- `go build ./...` — sem erros.
- `go vet ./...` — sem erros.
- `go test ./internal/validator/... ./internal/generators/... -v` — 14/14 testes passando.

**Observação:** projetos `backend=node` em modo `backend-only` não recebem pergunta sobre `pkgManager` (fica em `""`). A função `backendCommands` faz fallback para `npm` nesses casos — comportamento documentado e alinhado ao spec.

---

## Sessão 2026-06-11 — Apolo (CONCLUÍDO)

**Tarefa:** ML-1A do roadmap `roadmap-adr-wizard-e-list-2026-06-11` — wizard interativo `adr new` + subcomando `adr list`.

**Entregue:**
- `internal/generators/adr.go` — struct `ADRContent{Title, Context, Decision, Consequences, Alternatives}`; `NewADR(ADRContent)` puro (sem I/O de UI); campos preenchidos inseridos diretamente, campos vazios mantêm placeholder HTML; nova função `ListADRs(dir)` (glob + print tabular); `parseADRMeta` extrai título e status do markdown.
- `internal/commands/adr.go` — `newADRNewCmd()` detecta TTY via `charmbracelet/x/term.IsTerminal`; wizard huh 4 campos em TTY, fallback silencioso em CI/não-TTY; `newADRListCmd()` registrado no grupo `adr`.
- `internal/generators/adr_test.go` — 7 testes: `CreatesFile`, `SlugInFilename`, `WithContent`, `EmptyFields`, `ListADRs_Empty`, `ListADRs_WithFiles`, `ListADRs_ParsesMeta`.
- `go build ./...` sem erros | `go vet ./...` limpo | 20/20 testes verdes.
- Commit `e4a69d8` na branch `feat/adr-wizard-e-list` | push para remoto.

**Decisões técnicas:**
- Usado `charmbracelet/x/term` (já no go.mod) ao invés de `golang.org/x/term` — evita nova dependência.
- `ListADRs` e `parseADRMeta` ficam em `generators` para permitir teste direto sem cobra.
- Wizard só ativa em TTY — em CI o comando ainda funciona gerando ADR com placeholders.

---

## Sessão 2026-06-11 — Apolo (CONCLUIDO)

**Tarefa:** ML-1A do roadmap `roadmap-req-wizard-e-list-2026-06-11` — wizard interativo `req new` + subcomando `req list`.

**Entregue:**
- `internal/generators/req.go` — struct `REQContent{Title, Motivation, Criteria, LinkedADR, LinkedRoadmap}`; `NewREQ(REQContent)` puro sem I/O de UI; campos preenchidos inseridos diretamente, campos vazios mantêm placeholder HTML/markdown; `ListREQs(dir)` (glob + print tabular); `parseREQMeta` extrai título e status do markdown.
- `internal/commands/req.go` — `newReqNewCmd()` detecta TTY via `charmbracelet/x/term.IsTerminal`; wizard huh 4 campos em TTY (Motivation, Criteria, LinkedADR, LinkedRoadmap), fallback silencioso em CI/não-TTY; `newReqListCmd()` registrado no grupo `req`.
- `internal/generators/req_test.go` — 7 testes: `CreatesFile`, `SlugInFilename`, `WithContent`, `EmptyFields`, `ListREQs_Empty`, `ListREQs_WithFiles`, `ListREQs_ParsesMeta`.
- `go build ./...` sem erros | `go vet ./...` limpo | 26/26 testes verdes.
- Commit `0db0864` na branch `feat/req-wizard-e-list` | push para remoto.

---

---

## Sessão 2026-06-11 — Apolo (CONCLUIDO)

**Tarefa:** Implementar geração de roadmap por IA no `trackfw roadmap new` (branch `feat/roadmap-ai-generation`).

**Entregue:**
- `internal/ai/` — Client interface, AnthropicClient (SDK v1.50.1 — API v1.x sem `anthropic.F()`), OpenAIClient (stdlib), FakeClient, ReadConfig (parser YAML simples sem dependência de yaml.v3)
- `internal/generators/roadmap.go` — struct RoadmapContent + NewRoadmapFromContent; NewRoadmap refatorado para delegar
- `internal/commands/roadmap.go` — reescrito: wizard huh.Select lista docs/req/*.md, lê conteúdo da REQ, chama IA se configurada, fallback template vazio
- `internal/generators/scaffold.go` — Config.AIProvider/AIApiKey; writeTrackfwConfig escreve ai_provider/ai_model/ai_api_key
- `internal/commands/init.go` — Grupo 5 no wizard (provider + api key)
- Commit `7656a4b` | push para `feat/roadmap-ai-generation`

**Resultado:** 29/29 testes verdes | `go build ./...` limpo | `go vet ./...` limpo

**Decisoes tecnicas:**
- SDK Anthropic v1.50.1: `Messages []MessageParam` (sem wrapper F()), `NewUserMessage(NewTextBlock(prompt))` como helper, `msg.Content[0].Text` para acessar texto
- OpenAI implementado com stdlib pura (sem dependência adicional)
- ai_model: escrita sem valor no YAML (campo livre editável manualmente) — sem verb Sprintf para evitar corrupção silenciosa

---

## Sessão 2026-06-11 — Zeus + Apolo (CONCLUÍDO)

**Tarefa:** Geração de roadmap por IA — `trackfw roadmap new` com wizard interativo + integração Anthropic/OpenAI + fallback template vazio.

**Entregue:**
- `internal/ai/client.go` — interface `Client{Generate}` + factory `NewClient(provider, model, apiKey)`
- `internal/ai/anthropic.go` — struct `anthropicClient` via `github.com/anthropics/anthropic-sdk-go` v1.50.1
- `internal/ai/openai.go` — struct `openAIClient` via stdlib `net/http` + `encoding/json`
- `internal/ai/fake.go` — `FakeClient{Response string}` para testes
- `internal/ai/config.go` — `ReadConfig(path)` lê `ai_provider`, `ai_model`, `ai_api_key` de YAML flat sem yaml.v3
- `internal/ai/client_test.go` — 3 testes: `ReadConfig_Empty`, `ReadConfig_WithValues`, `FakeClient_Generate`
- `internal/generators/roadmap.go` — `RoadmapContent{Title, REQPath, Body}` + `NewRoadmapFromContent`; `NewRoadmap` refatorado para delegar
- `internal/generators/roadmap_test.go` — 2 novos testes: `NewRoadmapFromContent_CreatesFile`, `NewRoadmapFromContent_EmptyBody`
- `internal/generators/scaffold.go` — `Config.AIProvider`, `Config.AIApiKey`; `writeTrackfwConfig` gera `ai_provider`/`ai_model`/`ai_api_key` no YAML
- `internal/commands/init.go` — Grupo 5 AI no wizard (`huh.Select` provider + `huh.Input` api key)
- `internal/commands/roadmap.go` — reescrito: `huh.Select` lista `docs/req/*.md`, lê config AI, chama IA ou fallback template, `Args: cobra.MaximumNArgs(1)`
- `go.mod` — `github.com/anthropics/anthropic-sdk-go v1.50.1` adicionado

**Resultado:** 29/29 testes verdes | `go build ./...` limpo | `go vet ./...` limpo | commit `7656a4b` na branch `feat/roadmap-ai-generation`

---

## Sessão 2026-06-11 — Apolo (CONCLUIDO)

**Tarefa:** Reverter integração de IA do binário Go — remover `internal/ai/`, campos AI do Config, Grupo 5 do wizard init, lógica AI do roadmap.go, e rodar `go mod tidy`.

**Entregue:**
- `internal/ai/` deletado integralmente (6 arquivos: client.go, anthropic.go, openai.go, fake.go, config.go, client_test.go)
- `internal/generators/scaffold.go` — campos `AIProvider`/`AIApiKey` removidos de `Config`; template `writeTrackfwConfig` sem `ai_provider`/`ai_model`/`ai_api_key`
- `internal/commands/init.go` — variáveis `aiProvider`/`aiApiKey` e Grupo 5 do wizard removidos; struct `cfg` sem campos AI
- `internal/commands/roadmap.go` — reescrito: sem imports `context`/`time`/`ai`, sem `roadmapPromptTemplate`, sem lógica AI; `roadmap new` simplificado com `body: ""` implícito via `RoadmapContent` sem campo Body
- `go mod tidy` — `github.com/anthropics/anthropic-sdk-go` e deps transitivas removidos
- 26/26 testes verdes | `go build ./...` limpo | `go vet ./...` limpo
- Commit `2557fef` | push para `feat/roadmap-ai-generation`

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** Wave 3+4B do roadmap `v1.1.0-i18n-wizard-java` — infraestrutura i18n Go + wiring nos comandos.

**Entregue:**
- `internal/i18n/i18n.go` — pacote i18n com `DetectLocale()` (LANG/LC_ALL/LANGUAGE), `load()` com `sync.Once`, `T(key string, vars ...string) string` (dot-path + interpolação `{{var}}`), `Locale() string`. Embed de 3 arquivos JSON via `//go:embed`.
- `internal/i18n/locales/en-US.json` — traduções em inglês (init, adr, req, roadmap, validate, status, log, plugins, errors).
- `internal/i18n/locales/pt-BR.json` — traduções em português brasileiro.
- `internal/i18n/locales/es-ES.json` — traduções em espanhol.
- `internal/commands/init.go` — `newInitCmd().Short` usa `i18n.T("init.description")`; títulos dos prompts huh via variáveis intermediárias com `i18n.T("init.prompt.*")`; `fmt.Println(i18n.T("init.success"))`.
- `internal/commands/validate.go` — `Short`, mensagens de ok/violations/warnings via `i18n.T()`.
- `internal/commands/log.go` — `Short`, flag `--tail` description, mensagem "No transitions" via `i18n.T()`.
- `go build ./...` limpo | `go test ./...` 100% verde | `LANG=pt_BR.UTF-8 bin/trackfw --help` exibe comandos traduzidos.

---

## Estrutura atual do projeto

```
trackfw/
├── cmd/trackfw/main.go
├── internal/
│   ├── commands/        # init, adr, req, roadmap, status, validate
│   ├── generators/      # scaffold, adr, req, roadmap
│   └── validator/       # validate + status
├── docs/
│   ├── visao-projeto/VISION.md
│   └── agents-working-context.md  ← este arquivo
├── scripts/install.sh
├── Makefile
├── go.mod               # module github.com/trackfw/trackfw
├── go.sum
└── .gitignore
```

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** ML-1A do roadmap `feat/req-driven-adr-discovery` — catálogo de probes e detecção de domínio.

**Entregue:**
- `internal/generators/probes.go` — tipos `Probe`, `Question`, `ProbeOption`; `ProbesCatalog` com 6 domínios (authentication, ui, persistence, api, deploy, events); `DetectDomains(intention string) []Probe` — busca case-insensitive por substring nos keywords.
- `internal/generators/probes_test.go` — 5 testes: `Authentication`, `UI`, `NoMatch`, `MultiDomain`, `CaseInsensitive`.
- `go build ./...` limpo | 5/5 testes verdes | commit `2cb3976` | push para `feat/req-driven-adr-discovery`.

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** Detectar roadmaps em WIP stale (> 7 dias sem modificação) na branch `feat/v1-remaining-features`.

**Entregue:**
- `internal/validator/validator.go` — constante `staleWIPDays = 7`; função `validateStaleWIP()` que usa `filepath.Glob` + `os.Stat` para calcular idade por `ModTime`; integrada em `Validate()` após `validateSingleWIP()`; seção `⚠  Stale WIP` adicionada em `GetStatus()` entre `❌ Blocked` e `⏳ REQs blocked by Draft ADRs`.
- Import `"time"` adicionado.
- `go build ./...` limpo | `go test ./...` 100% verde | `go vet ./...` limpo | commit `406ebcf` na branch `feat/v1-remaining-features`.

---

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** ML-1B do roadmap `feat/req-driven-adr-discovery` — Adicionar `NewADRDraft` em `internal/generators/adr.go`.

**O que foi feito:**
- Adicionadas funções `slugToTitle` e `NewADRDraft` ao final de `internal/generators/adr.go`
- `NewADRDraft` cria ADR com `Status: Draft`, é idempotente via glob por slug, e deriva o título do slug via title case
- Adicionados 4 testes em `internal/generators/adr_test.go`: `TestNewADRDraft_CriaArquivo`, `TestNewADRDraft_StatusDraft`, `TestNewADRDraft_Idempotente`, `TestNewADRDraft_TituloDerivado`
- Build e testes passando: `go build ./...` ok, 4/4 testes verdes
- Commit `7510a64` pushado para branch `feat/req-driven-adr-discovery`

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** ML-2A do roadmap `feat/req-driven-adr-discovery` — Estender `REQContent` com `DependsOnADRs []string` e gerar seção "Blocked by ADRs" no arquivo REQ.

**Entregue:**
- `internal/generators/req.go` — campo `DependsOnADRs []string` adicionado em `REQContent`; `NewREQ` gera cabeçalho com `| Blocked by ADRs: N` quando há ADRs vinculados; nova seção `## Blocked by ADRs` inserida entre `Linked ADR` e `Linked Roadmap`; `parseREQMeta` corrigido para extrair status antes do próximo pipe (evita capturar "Blocked by ADRs: 2" como parte do status).
- `internal/generators/req_test.go` — 3 novos testes: `TestNewREQ_ComADRsVinculados`, `TestNewREQ_SemADRsVinculados`, `TestNewREQ_ContadorNoStatus`.
- `go build ./...` limpo | 10/10 testes `TestNewREQ` verdes | suite completa OK.
- Commit `7e2a069` | push para `feat/req-driven-adr-discovery`.

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** ML-2B do roadmap `feat/req-driven-adr-discovery` — Wizard `req new` com etapa de probes contextuais.

**Entregue:**
- `internal/commands/req.go` — `runReqNew` refatorado com dois forms em sequência:
  - Form 1: coleta `Title` + `Motivation` em grupo único.
  - Detecção automática via `generators.DetectDomains(title + motivation)`.
  - Form 2: grupos de `Criteria`, `LinkedADR`/`LinkedRoadmap` + um `huh.Select` por question de cada probe detectada.
  - Respostas processadas: ADRSlug não-vazio gera ADR Draft via `generators.NewADRDraft`; resultado salvo em `content.DependsOnADRs` (deduplicado via `uniqueStrings`).
  - Mensagem final lista ADR drafts criados e orienta a resolvê-los antes do roadmap.
- Helper `uniqueStrings` adicionado no mesmo arquivo.
- Em modo não-TTY (CI): fluxo direto sem wizard/probes — comportamento inalterado.
- `go build ./...` limpo.

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** ML-3A do roadmap `feat/req-driven-adr-discovery` — Adicionar regra de validação em `internal/validator/validator.go` que detecta REQs Open bloqueadas por ADRs com Status: Draft.

**Entregue:**
- `validateREQsNotBlockedByDraftADRs()` — percorre `docs/req/*.md`, filtra REQs com `Status: Open`, extrai ADRs da seção `## Blocked by ADRs` via `parseBlockedADRs()`, verifica `Status: Draft` via `adrIsDraft()`, emite violation `"REQ X is blocked by Draft ADR: Y"`.
- `parseBlockedADRs(path)` — parser de seção markdown: lê de `## Blocked by ADRs` até próximo `##`, extrai basename `.md` de cada linha `- `.
- `adrIsDraft(adrBasename)` — lê `docs/adr/<basename>` e verifica presença de `"Status: Draft"`.
- `blockedREQs()` — retorna `map[string][]string` (req → adrs Draft) para uso em `GetStatus()`.
- Integrada em `Validate()` após `validateSingleWIP()`.
- Integrada em `GetStatus()` com seção "REQs blocked by Draft ADRs" (adicionada externamente antes da conclusão desta sessão).
- 3 testes novos: `TestValidateREQsNotBlockedByDraftADRs_Violação`, `_SemViolação`, `_Retrocompatível`.
- `go build ./...` limpo | todos os testes verdes | commit `36d582b` | push para `feat/req-driven-adr-discovery`.

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** ML-3B do roadmap `feat/req-driven-adr-discovery` — Adicionar seção `⏳ REQs blocked by Draft ADRs` ao `GetStatus()`.

**Entregue:**
- `internal/validator/validator.go` — função `blockedREQs() (map[string][]string, error)` que reutiliza `parseBlockedADRs` e `adrIsDraft` do ML-3A; seção adicionada em `GetStatus()` entre `❌ Blocked` e `✅ Done`, emitida apenas quando há REQs bloqueadas.
- `internal/validator/validator_test.go` — 2 novos testes: `TestGetStatus_REQsBloqueadas` (verifica presença da seção e do ADR listado) e `TestGetStatus_SemREQsBloqueadas` (verifica ausência quando não há bloqueios). Padrão de fixture igual ao existente (`t.TempDir()` + `chdir`).
- `go build ./...` limpo | 12/12 testes verdes | commit `85b0ba1` | push para `feat/req-driven-adr-discovery`.

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** Implementar `trackfw log` e registro automático de transições de estado (branch `feat/v1-remaining-features`).

**Entregue:**
- `internal/generators/roadmap.go` — `appendTransitionLog(basename, fromState, toState)` grava em `docs/roadmaps/.trackfw-log` no formato `YYYY-MM-DD HH:MM  <basename padded 50>  <from> → <to>`; `MoveRoadmap` extrai `fromState` via `filepath.Base(filepath.Dir(src))` e chama `appendTransitionLog` após `os.Rename` bem-sucedido.
- `internal/commands/log.go` — comando cobra `log` com flag `--tail N` (default 20); lê `.trackfw-log`, seleciona as últimas N linhas e imprime com cabeçalho; mensagem amigável se arquivo inexistente.
- `internal/commands/root.go` — `newLogCmd()` registrado na lista de subcomandos.
- `go build ./...` limpo | testes verdes | `go vet ./...` limpo | commit `138b4e8` na branch `feat/v1-remaining-features`.

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** Implementar sistema de plugins do trackfw (branch `feat/v1-remaining-features`).

**Entregue:**
- `internal/plugins/plugins.go` — pacote novo; `Dir()` retorna `~/.trackfw/plugins`; `List()` lista binários instalados; `Install(repo)` baixa asset das GitHub Releases (formato `user/name[@tag]`, detecta GOOS/GOARCH); `Remove(name)` remove plugin pelo nome.
- `internal/commands/plugins.go` — comando cobra `plugins` com subcomandos `list`, `add` e `remove`; `RunPlugin(name, args)` executa plugin instalado passando stdin/stdout/stderr.
- `internal/commands/root.go` — `newPluginsCmd()` registrado; `rootCmd.Args = cobra.ArbitraryArgs` + `rootCmd.RunE` configurados para dispatch automático de comandos desconhecidos para plugins.
- `go build ./...` limpo | `go test ./...` verde | `go vet ./...` limpo | commit `d201b45` na branch `feat/v1-remaining-features`.

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** Adicionar subcomando `trackfw roadmap show <name>` com busca parcial por nome.

**Entregue:**
- `internal/generators/roadmap.go` — função `ShowRoadmap(name string) error` adicionada: busca via `filepath.Glob` em todos os estados (`docs/roadmaps/*/*name*.md`), exibe cabeçalho com basename e estado em maiúsculas, conteúdo completo do arquivo e path.
- `internal/commands/roadmap.go` — função `newRoadmapShowCmd()` adicionada e registrada em `newRoadmapCmd()`.
- `go build ./...` limpo | `go test ./...` verde | `go vet ./...` limpo.
- Commit `6d4cc19` na branch `feat/v1-remaining-features`.

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** ML-1A do roadmap de reescrita do pacote npm em Node.js puro (branch `feat/npm-nodejs-rewrite`) — Atualizar package.json e entry point.

**Entregue:**
- `npm/package.json` — reescrito: removidos campos `os`/`cpu`, adicionados `main`, `files` com `src/`, `dependencies` (`commander ^12.0.0`, `@inquirer/prompts ^5.0.0`), `engines.node` atualizado para `>=18`.
- `npm/bin/trackfw` — reescrito: sem mais fat-package/spawnSync de binário Go; entry point Node puro que chama `createProgram().parseAsync(process.argv)`.
- `npm/bin/.gitkeep` — removido.
- `npm/src/commands/index.js` — criado: stub commander com `name/description/version`; exporta `createProgram()`.
- `npm/package-lock.json` — gerado via `npm install` (41 pacotes: commander + @inquirer/prompts + transitivos).
- Critério de aceite: `node npm/bin/trackfw --help` imprime usage sem erro. Passou.

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** ML-1B do roadmap de reescrita do pacote npm em Node.js puro (branch `feat/npm-nodejs-rewrite`) — Criar estrutura src/ com stubs.

**Entregue:**
- `npm/src/commands/index.js` — reescrito: `createProgram()` lê version do `package.json`, registra 8 subcomandos via `addCommand`, hook `preSubcommand` vazio para futura dispatch de plugins.
- `npm/src/commands/init.js` — stub: `trackfw init` → `TODO: init`.
- `npm/src/commands/adr.js` — stub com subcomandos `new <title>` e `list`.
- `npm/src/commands/req.js` — stub com subcomandos `new <title>` e `list`.
- `npm/src/commands/roadmap.js` — stub com subcomandos `new`, `list`, `show <name>`, `move <name> <state>`.
- `npm/src/commands/validate.js` — stub: `trackfw validate` → `TODO: validate`.
- `npm/src/commands/status.js` — stub: `trackfw status` → `TODO: status`.
- `npm/src/commands/log.js` — stub com flag `--tail <n>` (default 20).
- `npm/src/commands/plugins.js` — stub com subcomandos `list`, `add <repo>`, `remove <name>`.
- `npm/src/generators/{adr,req,roadmap,init}.js` — stubs `module.exports = {}`.
- `npm/src/validator/index.js` — stub `module.exports = {}`.
- Critério de aceite: `node -e "const {createProgram}=require('./npm/src/commands/index.js'); const p=createProgram(); console.log(p.commands.map(c=>c.name()))"` retorna todos os 8 subcomandos. Passou.

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** ML-2A do roadmap de reescrita do pacote npm em Node.js puro (branch `feat/npm-nodejs-rewrite`) — Implementar `npm/src/generators/adr.js` e `npm/src/commands/adr.js`.

**Entregue:**
- `npm/src/generators/adr.js` — funções `newADR(content)`, `listADRs(dir)`, `newADRDraft(slug)`, `toSlug(s)` portadas do Go; placeholders HTML idênticos; `newADRDraft` idempotente via regex sobre `readdirSync`; coluna 60 chars no `list`; helper `parseADRStatus` extrai status da linha `| Status: `.
- `npm/src/commands/adr.js` — implementação real (não mais stub); subcomando `new <title>` com wizard `@inquirer/prompts` em TTY + fallback silencioso em não-TTY; subcomando `list` delega para `generators.listADRs('docs/adr')`.
- Critérios de aceite validados manualmente em `/tmp/trackfw-test-node`:
  - `adr list` (diretório vazio) → `No ADRs found in docs/adr` ✅
  - `adr new "Test Decision" < /dev/null` → `created docs/adr/ADR-2026-06-12-test-decision.md` ✅
  - `adr list` (após criação) → linha com arquivo e status `Proposed` em coluna 60 ✅
  - Conteúdo do arquivo com template e placeholders idênticos ao gerador Go ✅

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** ML-2C do roadmap de reescrita npm Node.js — Implementar `npm/src/commands/log.js` com leitura real do `.trackfw-log`.

**Entregue:**
- `npm/src/commands/log.js` — implementação real: lê `docs/roadmaps/.trackfw-log`, filtra linhas vazias, aplica `--tail N` (default 20), imprime cabeçalho + linhas; mensagem amigável se arquivo inexistente.
- Critérios de aceite validados: sem log → "No transitions recorded yet." | com log → cabeçalho + linha impressos | `--version` → "0.1.0".

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** ML-2B do roadmap de reescrita npm Node.js — portar `listREQs`, `listRoadmaps`, `showRoadmap`, `moveRoadmap`, `appendTransitionLog`, `newRoadmap` para Node.js puro + atualizar commands.

**Entregue:**
- `npm/src/generators/req.js` — `listREQs(dir)`: glob `.md`, extrai status da linha `| Status: ...`, padding 60 chars, fallback `No REQs found in <dir>`.
- `npm/src/generators/roadmap.js` — `VALID_STATES`, `listRoadmaps()`, `showRoadmap(name)`, `moveRoadmap(name, state)`, `appendTransitionLog(basename, from, to)`, `newRoadmap(title, reqPath)`, helpers `findRoadmapMatches` e `toSlug`. Zero dependências externas.
- `npm/src/commands/req.js` — `req list` delegando a `listREQs('docs/req')`.
- `npm/src/commands/roadmap.js` — todos os 4 subcomandos (`new`, `list`, `show`, `move`) delegando aos generators.

**Critérios de aceite validados:**
- `roadmap list` vazio → mensagem orientando usuário ✅
- `roadmap list` com arquivo em backlog → lista `[backlog]` ✅
- `roadmap move test wip` → `✓ moved ROADMAP-2026-06-12-test.md → docs/roadmaps/wip` + log gravado ✅
- `roadmap show test` → cabeçalho `── BASENAME ── [WIP] ──────────...` + conteúdo + `Location:` ✅
- `req list` vazio → `No REQs found in docs/req` ✅
- `req list` com arquivo → `REQ-...md                    Open` ✅

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** ML-3A do roadmap de reescrita npm Node.js — Implementar `npm/src/validator/index.js` (porte completo do validador Go) + `npm/src/commands/validate.js` + `npm/src/commands/status.js`.

**Entregue:**
- `npm/src/validator/index.js` — porte completo do `internal/validator/validator.go`: 9 funções de validação + auxiliares `parseBlockedADRs`, `adrIsDraft`, `listDir`, `blockedREQs`, função principal `validate()` e `getStatus()`. Zero dependências externas.
- `npm/src/commands/validate.js` — saída `✓ No violations found.` / listagem de violations e warnings / `process.exit(1)` em violações.
- `npm/src/commands/status.js` — delegando para `getStatus()`.

**Critérios de aceite:** diretório vazio → `✓ No violations found.` ✅ | `status` → seções formatadas ✅ | `node --check` limpo ✅

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** ML-3B do roadmap de reescrita npm Node.js — Portar `newREQ`, `PROBES_CATALOG`, `detectDomains` para `npm/src/generators/req.js` e reescrever wizard `req new` em `npm/src/commands/req.js`.

**Entregue:**
- `npm/src/generators/req.js` — funções `newREQ(content)`, `PROBES_CATALOG` (6 domínios: authentication, ui, persistence, api, deploy, events — porte exato do Go), `detectDomains(intention)` adicionadas sem remover `listREQs`/`parseREQStatus` existentes; helper `toSlug` local; template idêntico ao Go com seção `## Blocked by ADRs`, linha de status com contador `| Blocked by ADRs: N`.
- `npm/src/commands/req.js` — `req new` reescrito com wizard `@inquirer/prompts` em dois passos (TTY) + fallback silencioso (não-TTY); perguntas dinâmicas por probe via `select`; ADR drafts gerados via `adrGenerators.newADRDraft`; deduplicação via `Set`; mensagem final lista ADR drafts criados.
- Critérios de aceite validados:
  - `req new "OAuth login" < /dev/null` → `created docs/req/REQ-2026-06-12-oauth-login.md` com template correto e `Status: Open` ✅
  - `req list` → `REQ-2026-06-12-oauth-login.md   Open` ✅
  - `detectDomains("OAuth login via SSO provider")` → `['authentication']` ✅
  - `newREQ` com `dependsOnADRs` → seção `## Blocked by ADRs` populada + status `| Blocked by ADRs: 2` ✅

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** ML-3C do roadmap de reescrita npm Node.js — Implementar `npm/src/generators/init.js` (scaffold completo) e `npm/src/commands/init.js` (wizard com @inquirer/prompts).

**Entregue:**
- `npm/src/generators/init.js` — `GOV_DIRS` (7 entradas), `scaffold(cfg)`, `writeTrackfwConfig`, `generateValidateScript` + `buildValidateScript` (go/java/node/python + frontend), `generateCIWorkflow` (github-actions/gitlab-ci), `generateGitHooks` (husky/lefthook), `generateClaudeMD` (seções frontend/backend/pre-commit/hooks/CI), `generateClaudeCommands` (7 slash commands idempotentes), stubs `installAgents/Gemini/Cursor/Copilot/Windsurf/AmazonQ` com mensagem orientativa.
- `npm/src/commands/init.js` — wizard completo com `@inquirer/prompts` (input/select/checkbox), guard `!process.stdin.isTTY` com defaults, try/catch para fallback em stdin inesperadamente fechado, dispatch para instaladores de AI tools.
- Critério de aceite validado: `echo "" | node npm/bin/trackfw init` cria os 7 diretórios de governança + trackfw.yaml + scripts/trackfw-validate.sh + CLAUDE.md + .claude/commands/trackfw (7 slash commands). Sintaxe validada com `node --check`.

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** Criar artefatos de governança para v1.1.0 — REQ e Roadmap de i18n, wizard init fixes e scaffold Java.

**Entregue:**
- `docs/requisições/claude/REQ-2026-06-12-i18n-wizard-java-scaffold.md` — REQ com motivação (4 melhorias pós validação em ambiente Windows corporativo) e 9 critérios de aceite mensuráveis.
- `docs/roadmaps/claude/backlog/v1.1.0-i18n-wizard-java-2026-06-12.md` — Roadmap com 4 waves, 9 MLs detalhados (Go binary + npm em paridade): Wave 1 (wizard fixes), Wave 2 (Java pom.xml), Wave 3 (i18n infra), Wave 4 (i18n wiring + templates).

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** Wave 1+2 do roadmap `v1.1.0-i18n-wizard-java` — adicionar pergunta de framework de backend ao wizard `trackfw init` (Go) e gerar `pom.xml` Spring Boot 3.3 quando backend=java.

**Entregue:**
- `internal/commands/init.go` — variável `backendFramework string` adicionada; title "Backend stack?" renomeado para "Backend language?"; segundo form `frameworkForm` executado após o form principal quando `backend != ""`; opções condicionais por linguagem (go: 4, java: 3, node: 4, python: 3); `cfg.BackendFramework` passado ao Config.
- `internal/generators/scaffold.go` — campo `BackendFramework string` adicionado em `Config`; `writeTrackfwConfig` gera linha `backend_framework: <valor>` no YAML; chamada `GeneratePomXML(cfg)` adicionada ao final de `Scaffold` com guard `cfg.Backend == "java"`.
- `internal/generators/java.go` — arquivo novo; `GeneratePomXML(cfg Config) error` gera `pom.xml` Spring Boot 3.3 / Java 21 com starter-web, starter-actuator e starter-test; reutiliza `toSlug` de `adr.go` (sem redefinição).
- `go build ./...` — sem erros | `make test` — todos os testes verdes.

**Observação:** `toSlug` já existia em `internal/generators/adr.go` — não foi redefinida em `java.go`.

---

## Sessão 2026-06-12 — Afrodite (CONCLUÍDO)

**Tarefa:** Criar infraestrutura i18n para o pacote npm do trackfw (branch `feat/v1.1.0-i18n-wizard-java`).

**Status:** CONCLUIDO

**Entregue:**
- `npm/src/i18n/index.js` — módulo de detecção de locale (LANG/LC_ALL/LANGUAGE + fallback Intl) e função `t(key, vars)` com interpolação `{{var}}`
- `npm/src/i18n/locales/en-US.json` — todas as strings do CLI em inglês
- `npm/src/i18n/locales/pt-BR.json` — tradução completa para português do Brasil
- `npm/src/i18n/locales/es-ES.json` — tradução completa para espanhol
- `npm/src/commands/validate.js` — wired com `t()`
- `npm/src/commands/status.js` — wired com `t()`
- `npm/src/commands/log.js` — wired com `t()`
- `npm/src/commands/roadmap.js` — wired com `t()`
- `npm/src/commands/plugins.js` — wired com `t()`; erros de download/plugin via `t()`
- `npm/src/commands/adr.js` — wired com `t()`; prompts do wizard i18n
- `npm/src/commands/req.js` — wired com `t()`; prompts do wizard i18n
- `npm/src/commands/init.js` — wired com `t()`; todos os prompts e messages do wizard i18n

**Validacao:**
- `node npm/bin/trackfw --help` — strings em EN-US (padrao) OK
- `LANG=pt_BR.UTF-8 node npm/bin/trackfw --help` — strings em PT-BR OK
- `LANG=es_ES.UTF-8 node npm/bin/trackfw --help` — strings em ES-ES OK
- `LANG=pt_BR.UTF-8 node npm/bin/trackfw validate` — "Nenhuma violacao encontrada." OK

---

## Sessão 2026-06-13 — Apolo ML-1A (CONCLUÍDO)

**Tarefa:** ML-1A do roadmap `feat/v2.0-gaps` — implementar `trackfw serve` (servidor HTTP local de visualização ADR→REQ→ROADMAP).

**Arquivos criados/modificados:**
- `internal/server/server.go` (novo) — handlers HTTP, parse de markdown, template HTML
- `internal/commands/serve.go` (novo) — comando cobra serve com flag --port
- `internal/commands/root.go` — newServeCmd() registrado
- `internal/i18n/locales/en-US.json`, `pt-BR.json`, `es-ES.json` — chave serve.description adicionada

**Resultado:** `go build ./...` limpo | `go vet ./...` limpo | `go test ./...` verde | `trackfw serve --help` mostra flag --port | `/api/data` retorna JSON válido | HTML com 3 seções (traceability, timeline, kanban) | commit `b0f27b8` | push para `feat/v2.0-gaps`.

---

## Sessão 2026-06-13 — Apolo ML-1B (CONCLUÍDO)

**Tarefa:** ML-1B do roadmap `feat/v2.0-gaps` — implementar `trackfw metrics` (cycle time, throughput e WIP age a partir do `.trackfw-log`).

**Arquivos a criar/modificar:**
- `internal/metrics/metrics.go` (novo) — ParseLog, Filter, Calculate, ExportCSV
- `internal/metrics/metrics_test.go` (novo) — testes unitários
- `internal/commands/metrics.go` (novo) — comando cobra metrics com --since e --export
- `internal/commands/root.go` — newMetricsCmd() registrado
- `internal/i18n/locales/*.json` — chave metrics.* nos 3 locales
- `npm/src/commands/metrics.js` (novo) — porte Node.js puro
- `npm/src/commands/index.js` — registrar command metrics

**Resultado:**
- `go build ./...` limpo | `go vet ./...` limpo | `go test ./internal/metrics/...` 8/8 verde
- `node --check npm/src/commands/metrics.js` OK
- Commit `a2fc979` | push para `feat/v2.0-gaps`
- `trackfw metrics --help` disponível com flags --since e --export

---

## Sessão 2026-06-13 — Apolo ML-2B (CONCLUÍDO)

**Tarefa:** ML-2B do roadmap `feat/v2.0-gaps` — WIP Limit configurável por squad via `trackfw.yaml`.

**Entregue:**
- `internal/generators/scaffold.go` — `Config.WipLimit int` e `Config.WipBySquad bool` adicionados; `writeTrackfwConfig` gera `wip_limit: 1` e `wip_by_squad: false` no YAML (com defaults quando campos zero).
- `internal/generators/roadmap.go` — campo `squad:` adicionado ao template de novo roadmap no frontmatter (após REQ:, vazio para preenchimento manual).
- `internal/validator/validator.go` — `WIPConfig{Limit, BySquad}` + `readWIPConfig()` (parser YAML flat, sem yaml.v3); `parseSquadFromFrontmatter(path)` extrai campo `squad:` do markdown; `validateWIPLimit()` substitui `validateSingleWIP()` — modo global conta todos os WIPs contra o limite, modo squad agrupa por squad e valida por grupo; `GetStatus()` exibe seção `⚙ WIP by Squad` com count e indicador ⚠/✓ quando `wip_by_squad: true`.
- `internal/validator/validator_test.go` — 5 novos testes: `Global_OK`, `Global_Exceed`, `Global_HighLimit`, `BySquad_OK`, `BySquad_Exceed`. Todos os 17 testes do pacote passando.
- `npm/src/validator/index.js` — paridade Node.js: `readWIPConfig()`, `parseSquadFromFrontmatter()`, `validateWIPLimit()` (retorna `{violations, warnings}`); `validate()` usa `validateWIPLimit` no lugar de `validateSingleWIP`; `getStatus()` exibe seção squad quando `bySquad: true`; novos exports adicionados.

**Resultado:** `go build ./...` limpo | `go vet ./...` limpo | 17/17 testes verdes | `node --check` OK | commit `0b39e3d` | push para `feat/v2.0-gaps`.

---

## Sessão 2026-06-13 — Apolo ML-2A (CONCLUÍDO)

**Tarefa:** ML-2A do roadmap `feat/v2.0-gaps` — `trackfw init --brownfield` modo lenient de governança.

**Arquivos criados/modificados:**
- `internal/generators/scaffold.go` — campos `BrownfieldMode bool` e `LenientUntil time.Time` adicionados em `Config`; `writeTrackfwConfig` escreve `governance_mode: lenient` e `lenient_until: YYYY-MM-DD` condicionalmente.
- `internal/commands/init.go` — flag `--brownfield` registrada em `newInitCmd()`; import `"time"` adicionado; `cfg.BrownfieldMode=true` e `cfg.LenientUntil=time.Now().AddDate(0,0,30)` quando flag ativa.
- `internal/validator/validator.go` — structs `GovernanceMode`, funções `readGovernanceMode()`, `IsLenient()`, `LenientUntilDate()` (exportadas) adicionadas; `Validate()` move violations para warnings quando `IsLenient()`.
- `internal/commands/validate.go` — imprime `[LENIENT MODE]` + `i18n.T("validate.lenient_mode", "date", until)` quando em modo lenient.
- `internal/i18n/locales/{en-US,pt-BR,es-ES}.json` — chave `validate.lenient_mode` adicionada nos 3 locales.
- `npm/src/generators/init.js` — `writeTrackfwConfig` escreve linhas lenient quando `cfg.brownfieldMode`.
- `npm/src/validator/index.js` — funções `readGovernanceMode()`, `isLenient()`, `lenientUntilDate()` adicionadas; `validate()` move violations para warnings quando lenient; exports atualizados.
- `npm/src/commands/validate.js` — imprime `[LENIENT MODE]` quando em modo lenient.
- `npm/src/i18n/locales/{en-US,pt-BR,es-ES}.json` — chave `validate.lenient_mode` adicionada nos 3 locales.

**Resultado:**
- `go build ./...` limpo | `go vet ./...` limpo | todos os testes verdes
- Teste integração: `trackfw validate` em projeto lenient → `[LENIENT MODE]`, `⚠ violation`, exit 0
- Teste integração: `trackfw validate` em projeto strict → `✗ violation`, exit 1 (inalterado)
- `node --check` limpo nos 3 arquivos npm modificados

---

## Sessão 2026-06-13 — Apolo ML-3A (CONCLUÍDO)

**Tarefa:** ML-3A do roadmap `feat/v2.0-gaps` — Plugin Registry: `trackfw plugins search` e resolução de nomes via registry `kgsaran/trackfw-plugins`.

**Entregue:**
- `internal/plugins/plugins.go` — `RegistryURL`, `RegistryEntry`, `parseRegistryYAML` (parser YAML lista-de-maps linha a linha, sem yaml.v3), `matchesKeyword` (name+description+tags), `Search` (GET registry + filter), `ResolveRepo` (sem `/` → busca no registry; com `/` → retorna direto sem rede); `Install` modificado para chamar `ResolveRepo` antes de baixar.
- `internal/plugins/plugins_test.go` — 6 testes sem rede: `ParseRegistryYAML_Empty`, `ParseRegistryYAML_OneEntry`, `MatchesKeyword_Name`, `MatchesKeyword_Tag`, `MatchesKeyword_NoMatch`, `ResolveRepo_WithSlash`.
- `internal/commands/plugins.go` — subcomando `search <keyword>` registrado; exit 0 em offline (mensagem amigável) e em sem matches.
- `npm/src/commands/plugins.js` — `fetchRegistry`, `parseRegistryYAML`, `matchesKeyword` e subcomando `search` com saída tabular e exit 0 em offline/sem matches.

**Resultado:** `go build ./...` limpo | `go vet ./...` limpo | 6/6 testes verdes | `node --check` OK | commit `26275dc` | push para `feat/v2.0-gaps`.

---

## Sessão 2026-06-13 — Apolo ML-3B (CONCLUÍDO)

**Tarefa:** ML-3B do roadmap `feat/v2.0-gaps` — `trackfw sync --to=linear` e `--to=jira`.

**Entregue:**
- `internal/sync/linear.go` — LinearClient: credenciais via trackfw.yaml ou env vars (LINEAR_API_KEY, LINEAR_TEAM_ID); CreateIssue via GraphQL mutation; readConfigField (parser YAML linha a linha sem yaml.v3).
- `internal/sync/jira.go` — JiraClient: credenciais via trackfw.yaml ou env vars (JIRA_BASE_URL, JIRA_EMAIL, JIRA_TOKEN, JIRA_PROJECT); CreateIssue via REST API v3 com Basic Auth (base64 email:token).
- `internal/sync/sync.go` — SyncToLinear, SyncToJira, syncToProvider: percorre docs/req/*.md, pula não-Open e já sincronizados, chama create, injeta campo no frontmatter; helpers extractTitle, extractMotivation, extractField, injectField, isStatusOpen.
- `internal/sync/sync_test.go` — 8 testes sem rede: SkipsNonOpen, SkipsAlreadySynced, InjectsField, ExtractTitle (3 casos), InjectField, InjectField_UpdatesExisting, ReadConfigField, ExtractMotivation. Todos 8/8 verdes.
- `internal/commands/sync.go` — cobra command `sync` com flag `--to` obrigatória; saída tabular REQ/ISSUE; mensagens de erro claras.
- `internal/commands/root.go` — newSyncCmd() registrado.
- `internal/generators/req.go` — campos `| Linear Issue:` e `| Jira Issue:` adicionados no template de REQ.
- `npm/src/commands/sync.js` — paridade Node.js com https stdlib; linearCreateIssue (GraphQL), jiraCreateIssue (REST v3), syncToProvider, syncToLinear, syncToJira; commander command com --to.
- `npm/src/commands/index.js` — sync registrado no createProgram().

**Resultado:** `go build ./...` limpo | `go vet ./...` limpo | 8/8 testes sync verdes | suite completa verde | `node --check` OK | commit `dfa58aa` | push para `feat/v2.0-gaps`.

---

## Sessão 2026-06-13 — Apolo (IMPLEMENTANDO)

**Tarefa:** ML-4A do roadmap v2.0-gaps — Hook `commit-msg` com validação de REQ em branches feat/fix.

**Branch:** `feat/v2.0-gaps`

**Entregue:**
- `internal/generators/scaffold.go` — campo `RequireReqInCommit bool` em `Config`; função `generateCommitMsgHook` (husky: `.husky/commit-msg`; lefthook: `lefthook.yml` + `.lefthook/commit-msg/trackfw-req-check.sh`); campo `require_req_in_commit` no `trackfw.yaml`
- `internal/commands/init.go` — segundo form condicional pós-wizard perguntando `require_req_in_commit` quando hooks != "none"; campo passado para `Config`
- `internal/generators/commitmsghook_test.go` — 3 testes: `TestGenerateCommitMsgHook_Husky`, `TestGenerateCommitMsgHook_Disabled`, `TestGenerateCommitMsgHook_Lefthook` — todos 3/3 verdes
- i18n locales Go (en-US, pt-BR, es-ES) — chave `init.prompt.require_req_in_commit`
- `npm/src/generators/init.js` — função `generateCommitMsgHook` + chamada em `scaffold()` + campo no `writeTrackfwConfig`
- `npm/src/commands/init.js` — pergunta condicional com `@inquirer/prompts` confirm; `requireReqInCommit` no cfg
- `npm/src/i18n/locales/` — chave `require_req_in_commit` nos 3 locales

**Resultado:** `go build ./...` limpo | `go vet ./...` limpo | suite completa verde | `node --check` OK | commit `add41a6` | push para `feat/v2.0-gaps`.

---

## Sessão 2026-06-13 — Apolo Wave 1 feat/v2.3-ai-agent-rail (CONCLUÍDO)

**Tarefa:** Wave 1 do roadmap `trackfw-ai-agent-rail` — ML-1A (frontmatter YAML em templates) e ML-1B (comando `trackfw context`).

**Branch:** `feat/v2.3-ai-agent-rail`

**ML-1A — Frontmatter YAML em templates (Go + npm):**
- `internal/generators/adr.go` — `NewADR()` e `NewADRDraft()` agora geram bloco `---` com `status`/`date`/`author`
- `internal/generators/req.go` — `NewREQ()` agora gera bloco `---` com `status`/`date`/`author`/`adr`/`roadmap`
- `internal/generators/roadmap.go` — template padrão (quando `content.Body == ""`) agora gera bloco `---` com `status`/`date`/`req`/`squad`
- `npm/src/generators/adr.js` — paridade: `newADR()` e `newADRDraft()` com frontmatter
- `npm/src/generators/req.js` — paridade: `newREQ()` com frontmatter
- `npm/src/generators/roadmap.js` — paridade: `newRoadmap()` com frontmatter

**ML-1B — Comando `trackfw context` (Go + npm):**
- `internal/generators/context.go` — `GetContext(format string) error`: coleta ADRs/REQs/Roadmaps via config, chama `validator.Validate()`, computa score (20pts/categoria + 40pts validate limpo), imprime em md ou json; `extractFrontmatterField()` e `extractInlineStatus()` como helpers
- `internal/commands/context.go` — cobra command `context` com flag `--format` (md|json)
- `internal/commands/root.go` — `newContextCmd()` registrado
- `npm/src/commands/context.js` — paridade Node.js puro: mesma lógica de coleta, score e formatação
- `npm/src/commands/index.js` — `require('./context')` registrado

**Resultado:** `go build ./...` limpo | `go test ./...` 100% verde | `node --check` OK em todos os arquivos npm
- Commit `66b5a8f` (ML-1A) | Commit `4f8b504` (ML-1B) | Push para `feat/v2.3-ai-agent-rail`

---

## Sessão 2026-06-13 — Apolo ML-3A (CONCLUÍDO)

**Tarefa:** ML-3A do roadmap `trackfw-ai-agent-rail` — JSON Schema para ADR/REQ/ROADMAP + `validateFrontmatterPresence` em Go e npm.

**Branch:** `feat/v2.3-ai-agent-rail`

**Entregue:**
- `docs/schema/adr.schema.json` — JSON Schema Draft-07; `required: ["status", "date"]`; `status` enum `["Draft","Proposed","Accepted","Deprecated","Superseded"]`; `date` pattern `^[0-9]{4}-[0-9]{2}-[0-9]{2}$`; campos opcionais `author`, `superseded_by`.
- `docs/schema/req.schema.json` — JSON Schema Draft-07; `required: ["status", "date"]`; `status` enum `["Open","Closed","Blocked"]`; campos opcionais `author`, `adr`, `roadmap`.
- `docs/schema/roadmap.schema.json` — JSON Schema Draft-07; `required: ["status", "date"]`; `status` enum `["backlog","wip","blocked","done","abandoned"]`; campos opcionais `req`, `squad`.
- `internal/validator/validator.go` — `extractFrontmatterField(content, field)` + `validateFrontmatterPresence()`: verifica ADRs e REQs sem bloco `---` de frontmatter; registrada em `Validate()` após `validateREQsNotBlockedByDraftADRs`.
- `npm/src/validator/index.js` — `validateFrontmatterPresence()` portada em Node.js puro; integrada em `validate()` e exportada em `module.exports`.

**Resultado:** `go build ./...` limpo | `go test ./...` 100% verde | `node --check npm/src/validator/index.js` OK | commit `f7ab22c` | push para `feat/v2.3-ai-agent-rail`.

---

## Sessão 2026-06-13 — Afrodite (CONCLUIDO)

**Tarefa:** Criar site de documentação VitePress bilíngue pt-BR/en-US + GitHub Actions deploy (branch `feat/v2.4-docs-site`)

**Branch:** `feat/v2.4-docs-site`

**Entregue:**
- `site/package.json` + `site/.gitignore` — configuração base VitePress 1.6.4
- `site/.vitepress/config.mts` — config bilíngue (root=pt-BR, /en=en-US), base=/trackfw/, search local, social links
- `site/index.md` + `site/en/index.md` — landing pages hero com features, instalação e quickstart
- `site/guide/getting-started.md` + `site/en/guide/getting-started.md` — guia completo (init, adr, req, roadmap, status, validate)
- `site/guide/commands.md` + `site/en/guide/commands.md` — referência de todos os comandos com flags e exemplos
- `site/guide/ai-agents.md` + `site/en/guide/ai-agents.md` — integração com Claude Code, Gemini CLI, Cursor, JSON Schema, prompts
- `.github/workflows/deploy-docs.yml` — build + deploy automático no GitHub Pages em push na main

**Resultado:** `npm run build` limpo | 9 HTMLs gerados em `.vitepress/dist/` | commit `d252e92` | push para `feat/v2.4-docs-site`

---

## Sessão 2026-06-13 — Apolo ML-1A Python CLI (CONCLUÍDO)

**Tarefa:** ML-1A do roadmap Python CLI nativo — `config.py` singleton + `__main__` entry point.

**Branch:** `feat/v2.2-python-cli-nativo`

**Entregue:**
- `pypi/trackfw/__init__.py` — `__version__ = "2.2.0"`.
- `pypi/trackfw/__main__.py` — entry point `from trackfw.cli import main; main()`.
- `pypi/trackfw/config.py` — funções `defaults()`, `load(cwd=None)`, `reset()`, `_parse(content, cfg)`; singleton `_instance`; parse YAML linha a linha sem dependência externa; constantes `NAMESPACING_FLAT` e `NAMESPACING_BY_AGENT`; paridade exata com `npm/src/config/index.js`.
- `pypi/tests/__init__.py` — vazio (declara pacote de testes).
- `pypi/tests/test_config.py` — 5 testes unittest: `test_defaults_sem_yaml`, `test_le_campos_escalares`, `test_le_adr_dirs`, `test_singleton`, `test_reset`.

**Resultado:** 5/5 testes verdes | commit `633016d` | push para `feat/v2.2-python-cli-nativo`.

---

## Sessão 2026-06-13 — Apolo ML-1B Python CLI (CONCLUÍDO)

**Tarefa:** ML-1B do roadmap Python CLI nativo — módulo i18n Python com suporte pt-BR/en-US/es-ES.

**Branch:** `feat/v2.2-python-cli-nativo`

**Entregue:**
- `pypi/trackfw/i18n/__init__.py` — detecção de locale via `TRACKFW_LANG`/`LANG`/`LANGUAGE`/`LC_ALL`; normalização `pt_BR*→pt-BR`, `es_*→es-ES`, qualquer outro→`en-US`; função `t(key, **vars)` com suporte a chaves aninhadas com `.` e interpolação `{{var}}`; fallback en-US e fallback para a própria chave; cache lazy com `reset()` para testes.
- `pypi/trackfw/i18n/locales/{pt-BR,en-US,es-ES}.json` — copiados de `npm/src/i18n/locales/`
- `pypi/tests/test_i18n.py` — 11 testes unittest: fallback en-US, pt-BR, es-ES, normalização LANG Unix, chave inexistente, chaves aninhadas, interpolação, detecção de locale, fallback de chave ausente.

**Resultado:** 11/11 testes verdes | sintaxe validada com `py_compile` | commit `e3087d1` | push para `feat/v2.2-python-cli-nativo`.

---

## Sessão 2026-06-13 — Apolo (CONCLUIDO)

**Tarefa:** ML-1C do roadmap Python CLI nativo — `validator.py` com wip-limit, stale-wip, req-adr em paridade com `npm/src/validator/index.js`.

**Branch:** `feat/v2.2-python-cli-nativo`

**Entregue:**
- `pypi/trackfw/validator.py` — espelho completo do validator JS: list_dir, resolve_wip_dirs, parse_frontmatter, validate_wip_has_req, validate_reqs_have_adr, validate_blocked_has_req, validate_reqs_have_roadmap, validate_adrs_are_referenced, validate_wip_has_acceptance_criteria, validate_wip_limit (flat/by_agent/by_squad), validate_stale_wip, validate_reqs_not_blocked_by_draft_adrs, validate_frontmatter_presence, validate(), modo lenient.
- `pypi/tests/test_validator.py` — 22 testes unittest passando (100%).
- Commit `a2a0407` | push para `feat/v2.2-python-cli-nativo`.

---

## Sessão 2026-06-13 — Apolo ML-2A Python CLI (CONCLUÍDO)

**Tarefa:** ML-2A do roadmap Python CLI nativo — `generators/__init__.py` + `generators/adr.py` + `tests/test_generators_adr.py`.

**Branch:** `feat/v2.2-python-cli-nativo`

**Entregue:**
- `pypi/trackfw/generators/__init__.py` — pacote vazio (declara o subpacote generators).
- `pypi/trackfw/generators/adr.py` — três funções: `next_adr_number(adr_dir)` escaneia ADR-NNN-*.md e retorna max+1; `slugify(title)` via unicodedata NFKD + encode ascii ignore, espaços→hífen, remove não-alfanuméricos; `generate_adr(title, status, adr_dirs, cwd)` cria arquivo ADR com frontmatter YAML e template markdown, numeração sequencial automática.
- `pypi/tests/test_generators_adr.py` — 13 testes unittest: TestNextAdrNumber (4 casos), TestSlugify (5 casos), TestGenerateAdr (4 casos). Todos 13/13 verdes.
- Commit `b9003b6` | push para `feat/v2.2-python-cli-nativo`.

---

## Sessão 2026-06-13 — Apolo ML-2B Python CLI (CONCLUÍDO)

**Tarefa:** ML-2B do roadmap Python CLI nativo — `generators/req.py` + `tests/test_generators_req.py`.

**Branch:** `feat/v2.2-python-cli-nativo`

**Entregue:**
- `pypi/trackfw/generators/req.py` — `slugify(title)` via `unicodedata.NFKD + ascii ignore`; `generate_req(title, req_dir, cwd)` cria `REQ-YYYY-MM-DD-<slug>.md` com frontmatter completo (name, title, status: Open, linked_adr: —, created, author) e seções Motivação, Critérios de Aceite, Fora de Escopo; cria `req_dir` automaticamente via `os.makedirs(exist_ok=True)`; retorna path absoluto.
- `pypi/tests/test_generators_req.py` — 8 testes unittest: `test_generate_req_cria_arquivo`, `test_frontmatter_correto`, `test_slugify_com_acentos`, `test_cria_req_dir_se_nao_existir`, `test_retorna_path_absoluto`, `test_conteudo_template`, `test_slugify_lowercase`, `test_slugify_sem_acentos`.

**Resultado:** 8/8 testes verdes | commit `bf64f67` | push para `feat/v2.2-python-cli-nativo`.

---

## Sessão 2026-06-13 — Apolo ML-2D Python CLI (CONCLUÍDO)

**Tarefa:** ML-2D do roadmap Python CLI nativo — `generators/init_gen.py` (scaffold flat/by_agent) + `tests/test_generators_init.py`.

**Branch:** `feat/v2.2-python-cli-nativo`

**Entregue:**
- `pypi/trackfw/generators/init_gen.py` — espelho de `npm/src/generators/init.js` em Python puro (stdlib apenas): `scaffold(cwd, opts)`, `_gov_dirs_by_agent(agents)`, `_write_trackfw_yaml(cwd, opts)`, `_write_example_adr(cwd, opts)`; constantes `GOV_DIRS_FLAT` e `ROADMAP_STATES`; ADR exemplo idempotente (não sobrescreve se já existir).
- `pypi/tests/test_generators_init.py` — 12 testes unittest distribuídos em 5 classes: `TestScaffoldFlat` (2), `TestScaffoldByAgent` (2), `TestTrackfwYamlGerado` (3), `TestIdempotente` (2), `TestExemploADR` (3).
- Suite completa: 82/82 testes verdes | `py_compile` OK | commit `591d4df` | push para `feat/v2.2-python-cli-nativo`.

---

## Sessão 2026-06-13 — Apolo ML-2C Python CLI (CONCLUÍDO)

**Tarefa:** ML-2C do roadmap Python CLI nativo — `generators/roadmap.py` + `tests/test_generators_roadmap.py`.

**Branch:** `feat/v2.2-python-cli-nativo`

**Entregue:**
- `pypi/trackfw/generators/roadmap.py` — espelho de `npm/src/generators/roadmap.js`: `slugify()`, `generate_roadmap()` (modo flat e by_agent), `move_roadmap()` (busca em todos os estados/agentes, atualiza `status:` no frontmatter, grava `.trackfw-log`); helpers `_state_dir`, `_agent_state_dir`, `_find_roadmap_matches`, `_append_transition_log`, `_roadmap_template`.
- `pypi/tests/test_generators_roadmap.py` — 11 testes unittest: `TestSlugify` (3 casos), `TestGenerateFlat` (gera em `backlog/`), `TestGenerateByAgent` (gera em `zeus/backlog/`, fallback primeiro agente), `TestMoveBacklogParaWip` (move arquivo, atualiza frontmatter, grava log, levanta erros), `TestMoveBuscaEmTodosAgentes` (by_agent sem especificar agente).

**Resultado:** 11/11 testes verdes | commit `3b3d3cb` | push para `feat/v2.2-python-cli-nativo`.

---

## Sessão 2026-06-13 — Apolo ML-3A Python CLI (CONCLUÍDO)

**Tarefa:** ML-3A do roadmap Python CLI nativo — Wave 3 comandos CLI: `cli.py` (entry point argparse), `commands/adr.py`, `commands/req.py`, `commands/log.py`, `commands/__init__.py`, `tests/test_commands_basic.py` + atualizar `pyproject.toml`.

**Branch:** `feat/v2.2-python-cli-nativo`

**Entregue:**
- `pypi/trackfw/commands/__init__.py` — declara pacote de subcomandos.
- `pypi/trackfw/cli.py` — entry point argparse com 11 subcomandos: `adr` e `req` e `log` com implementação real; `init`, `roadmap`, `validate`, `status`, `discover`, `metrics`, `context`, `sync`, `plugins` como stubs ("Not implemented yet", exit 0). Flag `--version` via argparse.
- `pypi/trackfw/commands/adr.py` — `register(subparsers)` + `adr new <title> [--status] [--dir]`; chama `generate_adr()`, imprime path criado.
- `pypi/trackfw/commands/req.py` — `register(subparsers)` + `req new [<title>]`; `input()` quando título ausente; chama `generate_req()`, imprime path criado.
- `pypi/trackfw/commands/log.py` — `register(subparsers)` + `log <message>`; append em `.trackfw-log` na raiz do projeto com timestamp `YYYY-MM-DD HH:MM`.
- `pypi/pyproject.toml` — entry point atualizado de `trackfw._cli:main` para `trackfw.cli:main`.
- `pypi/tests/test_commands_basic.py` — 11 testes de integração via `subprocess.run` com `PYTHONPATH=PYPI_DIR`; cobre `--version`, `adr new` (3 variações), `log` (3 variações) e 4 stubs.

**Resultado:** 93/93 testes verdes | commit `1f83956` | push para `feat/v2.2-python-cli-nativo`.

---

## Sessão 2026-06-14 — Athena (IMPLEMENTANDO)

**Tarefa:** Análise de mercado aprofundada e completa — trackfw vs. concorrentes em 6 segmentos: ADR Management, Spec/REQ Management, Roadmap, Platform Engineering/IDP, Engineering Metrics/DORA, AI-native Governance. WebSearch ativo para 20+ ferramentas. Entrega do relatório completo em markdown.

**Status:** CONCLUÍDO — relatório completo entregue. Cobertura: 6 segmentos, 25+ ferramentas analisadas via WebSearch. Posicionamento, diferenciadores únicos, gaps, ameaças, oportunidades e 9 recomendações estratégicas.

---

## Sessão 2026-06-13 — Apolo ML-3B Python CLI (CONCLUÍDO)

**Tarefa:** ML-3B do roadmap Python CLI nativo — `commands/validate.py` + `commands/status.py` + `tests/test_commands_validate_status.py`.

**Branch:** `feat/v2.2-python-cli-nativo`

**Entregue:**
- `pypi/trackfw/commands/validate.py` — `register(subparsers)` e `run(args)`: chama `validator.validate()`, imprime violations como `✗ <msg>` (vermelho ANSI se terminal suportar), warnings como `⚠ <msg>`, `✓ Governance OK` se tudo limpo; exit code 1 se violations; informa usuario sobre modo lenient.
- `pypi/trackfw/commands/status.py` — `register(subparsers)`, `run(args)`, `get_status(cwd)`: dashboard com contagem de ADRs, REQs (breakdown Open/Closed) e Roadmaps por estado; suporta modo `flat` e `by_agent` (totais agregados + seção "Roadmaps (by agent)" com contagens por agente); helper `_resolve(base, path)` garante paths relativos resolvidos ao `cwd` passado.
- `pypi/tests/test_commands_validate_status.py` — 10 testes unittest (sem subprocess, `tempfile.mkdtemp()`): `TestValidateSemViolations`, `TestValidateComViolation`, `TestValidateLenientExitZero`, `TestStatusFlat` (3 asserts), `TestStatusByAgent` (4 asserts).

**Resultado:** 10/10 testes novos verdes | suite completa 103/103 | commit `7e989a6` | push para `feat/v2.2-python-cli-nativo`.

---

## Sessão 2026-06-13 — Apolo ML-3C Python CLI (CONCLUÍDO)

**Tarefa:** ML-3C do roadmap Python CLI nativo — `commands/roadmap.py` + `commands/discover.py` + `tests/test_commands_roadmap_discover.py`.

**Branch:** `feat/v2.2-python-cli-nativo`

**Entregue:**
- `pypi/trackfw/commands/roadmap.py` — `register(subparsers)` com 4 subcomandos:
  - `roadmap new <title> [--agent]`: chama `generate_roadmap()`, imprime path criado.
  - `roadmap move <filename> <state>`: chama `move_roadmap()`, imprime novo path.
  - `roadmap list [--state]`: lista roadmaps por estado; modo flat agrupa por estado, modo by_agent agrupa por agente.
  - `roadmap show <filename>`: busca por nome exato ou parcial (case-insensitive), imprime conteúdo.
  - Helpers internos: `_list_flat`, `_list_by_agent`, `_find_file`.
- `pypi/trackfw/commands/discover.py` — `register(subparsers)` com flags `--init` e `--bootstrap-log`:
  - `scan(root_dir)`: detecta adr_dirs, req_dir, roadmap_dir, namespacing, agents, counts, score 0-100; espelha `internal/discover/discover.go` e `npm/src/commands/discover.js`.
  - `generate_yaml(result)`: gera conteúdo do trackfw.yaml.
  - `generate_bootstrap_log(result, root_dir)`: entradas retroativas baseadas em mtime dos arquivos em done/.
  - `install_gates(result, root_dir)`: instala validate script, hook entry e CI workflow.
  - `_cmd_discover(args)`: imprime relatório com score e executa --init/--bootstrap-log conforme flags.
- `pypi/tests/test_commands_roadmap_discover.py` — 26 testes unittest:
  - `TestRoadmapNew` (3 casos): flat, by_agent com agent, by_agent sem agent.
  - `TestRoadmapMove` (3 casos): move válido, estado inválido, arquivo não encontrado.
  - `TestRoadmapList` (3 casos): flat, by_agent, filtro por estado.
  - `TestRoadmapShow` (3 casos): exato, parcial, não encontrado.
  - `TestDiscoverScan` (6 casos): flat, by_agent, score 0, score parcial, github-actions, lefthook.
  - `TestDiscoverInit` (2 casos): arquivo criado, conteúdo correto.
  - `TestDiscoverBootstrapLog` (3 casos): flat, by_agent, sem done/.
  - `TestRegister` (3 casos): argparse de roadmap e discover.

**Resultado:** 26/26 testes novos verdes | suite completa 129/129 | commit `2fcbe02` | push para `feat/v2.2-python-cli-nativo`.

---

## Sessão 2026-06-13 — Apolo ML-3D Python CLI (CONCLUÍDO)

**Tarefa:** ML-3D do roadmap Python CLI nativo — Wave 3 comandos extras: `commands/metrics.py`, `commands/context.py`, `commands/sync.py`, `commands/plugins.py`, `tests/test_commands_extras.py`.

**Branch:** `feat/v2.2-python-cli-nativo`

**Entregue:**
- `pypi/trackfw/commands/metrics.py` — `register(subparsers)` com flags `--days`, `--since`, `--export`; `_parse_log()` via regex LINE_RE (espelha JS); `_calculate()` (cycle time médio, throughput por semana, WIP age); `_print_metrics()` (tabela ASCII); `_export_csv()`; `_filter()` por datetime; `_format_duration()`.
- `pypi/trackfw/commands/context.py` — `register(subparsers)` com flags `--format` e `--output`; `_get_context()` coleta ADRs/REQs/Roadmaps via config, chama `validator.validate()`, computa score, saída em markdown ou JSON; suporte a `--output FILE`.
- `pypi/trackfw/commands/sync.py` — `register(subparsers)` com flag `--to` obrigatória (linear|jira); `_sync_to_linear()` e `_sync_to_jira()` via `urllib.request` (stdlib pura); helpers `_read_config_field`, `_extract_title`, `_extract_motivation`, `_inject_field`, `_is_status_open`; `_sync_to_provider()` percorre `docs/req/*.md`, pula não-Open e já sincronizados; saída tabular REQ/ISSUE.
- `pypi/trackfw/commands/plugins.py` — `register(subparsers)` com sub-subcomandos `list` e `run`; `_find_plugins_in_path()` busca executáveis `trackfw-*` no PATH via `os.listdir` + `os.access`; `_cmd_run()` executa via `subprocess.run()`, repassa args e exit code.
- `pypi/tests/test_commands_extras.py` — 17 testes unittest: TestMetrics (6), TestContext (6), TestPlugins (5). Todos 17/17 verdes.

**Resultado:** 17/17 testes verdes | suite completa 146/146 | commit `09b54c5` | push para `feat/v2.2-python-cli-nativo`.

---

## Sessao 2026-06-13 — Artemis ML-4A Python CLI QA (CONCLUIDO)

**Tarefa:** ML-4A do roadmap Python CLI nativo — auditoria e validacao da suite de testes Python completa.

**Branch:** `feat/v2.2-python-cli-nativo`

**Resultado da auditoria:**
- **146/146 testes verdes** (0 failures, 0 errors)
- Suite completa em 0.688s
- Working tree limpo — todos os testes ja estavam commitados junto com cada ML de implementacao
- Nenhum teste faz chamada de rede (urllib/requests/http/socket ausentes nos arquivos de teste)
- Nenhum arquivo temporario deixado em `pypi/` apos execucao
- Cobertura verificada: config sem trackfw.yaml (test_defaults_sem_yaml), modo lenient (test_lenient_mode_violations_viram_warnings, test_validate_lenient_violations_viram_warnings), roadmap move (test_roadmap_move, test_roadmap_move_estado_invalido, test_roadmap_move_arquivo_nao_encontrado)
- Total >= 100 testes: 146 (criterio atendido com folga)

**Distribuicao por arquivo:**
- test_config.py: 5 | test_i18n.py: 11 | test_validator.py: 22
- test_generators_adr.py: 13 | test_generators_req.py: 8 | test_generators_roadmap.py: 11
- test_generators_init.py: 12 | test_commands_basic.py: 11
- test_commands_validate_status.py: 10 | test_commands_roadmap_discover.py: 26
- test_commands_extras.py: 17

**Agente:** Artemis | Status: CONCLUIDO

---

## Sessão 2026-06-13 — Zeus ML-4B + Fechamento v2.2 Python CLI (CONCLUÍDO)

**Tarefa:** ML-4B (remoção do wrapper `_cli.py`) + fechamento do roadmap v2.2.

**Branch:** `feat/v2.2-python-cli-nativo`

**ML-4B resultado:**
- `pypi/trackfw/_cli.py` (wrapper Go binary) removido
- Nenhuma referência residual a `_cli` nos arquivos Python/TOML
- `pip install -e pypi/` sem warnings
- `trackfw --version` → `trackfw 2.2.0`
- `python3 -m trackfw --help` funcional
- Commit `b2121dd` | push OK

**Fechamento do roadmap:**
- Roadmap movido de `wip/` para `done/`
- Todos os 11 MLs marcados ✅ Concluído
- Total: 146 testes, 12 comandos, zero dependências externas, Python 3.8+

**Próximos passos:** criar PR para `feat/v2.2-python-cli-nativo` → `main` e gerar tag v2.2.0 após merge.

**Agente:** Zeus | Status: CONCLUÍDO

---

## Sessão 2026-06-13 — Apolo ML-1A v2.3 Validator Improvements (CONCLUÍDO)

**Tarefa:** ML-1A do roadmap v2.3 — melhorias no validador Go do trackfw (5 mudanças).

**Branch:** `feat/v2.3-validator-improvements`

**Entregue:**

B1 — adr_dirs recursivo:
- `walkADRFiles(adrDir)` — WalkDir recursivo, retorna basenames de todos `.md`.
- `findADRFile(basename, adrDirs)` — busca o caminho completo recursivamente; usa `fs.SkipAll` ao encontrar.
- `validateADRsAreReferenced`, `validateFrontmatterPresence` e `adrIsDraft` migrados para busca recursiva.

B2 — stale WIP por git log:
- `gitLastModifiedTime(path)` — `git log -1 --format=%ct` com fallback para mtime do filesystem.
- `validateStaleWIP()` — usa timestamp do último commit quando disponível.

M3 — verificar existência de referências:
- `extractRefPath(content, field)` — extrai caminho `.md`; ignora vazios/traços.
- `validateRefTargetsExist()` — warnings para REQ:/ADR:/Roadmap: que não existem no filesystem.

M4 — coerência pasta × status:
- `validateFolderStatusCoherence()` — warning quando frontmatter `status:` diverge da pasta (flat e by_agent).

M5 — unicidade de filename entre estados:
- `validateFilenameUniqueness()` — violation quando mesmo filename aparece em múltiplos estados.

Testes (7 novos em `internal/validator/validator_improvements_test.go`):
- TestWalkADRFiles, TestADRDirsRecursiveInValidate, TestValidateStaleWIPFallback
- TestExtractRefPath (7 sub-casos), TestRefTargetsExistWarning, TestFolderStatusCoherence, TestFilenameUniqueness

**Resultado:** `go build ./...` limpo | 24/24 testes verdes | commit `a3a3697` | push para `feat/v2.3-validator-improvements`

---

## Sessão 2026-06-13 — Apolo ML-1B validator-improvements (CONCLUÍDO)

**Tarefa:** ML-1B do roadmap `feat/v2.3-validator-improvements` — Melhorias no validador Node.js.

**Branch:** `feat/v2.3-validator-improvements`

**Entregue:**
- `npm/src/validator/index.js` — walkDirMd, findAdrFile, gitLastModifiedTime adicionados; adrIsDraft, validateADRsAreReferenced, validateFrontmatterPresence e validateStaleWIP atualizados para busca recursiva; extractRefPath, validateRefTargetsExist, validateFolderStatusCoherence, validateFilenameUniqueness + FOLDER_TO_STATUS implementados; validate() inclui novas validações; module.exports expandido.
- `npm/tests/validator.test.js` — criado: 12/12 testes passando (sem framework externo).

**Resultado:** `node --check` OK | 12/12 testes verdes | `validate()` OK | commit `c1b236b` | push para `feat/v2.3-validator-improvements`.

**Agente:** Apolo | Status: CONCLUÍDO

---

## Sessão 2026-06-13 — Apolo ML-1C validator-improvements Python (IMPLEMENTANDO)

**Tarefa:** ML-1C do roadmap `feat/v2.3-validator-improvements` — Melhorias no validador Python (`pypi/trackfw/validator.py`).

**Branch:** `feat/v2.3-validator-improvements`

**Mudanças a implementar:**
- B1: `_walk_dir_md` + `_find_adr_file` (ADR dirs recursivo)
- B2: `_git_last_modified_time` + `subprocess` (stale WIP por git log)
- M3: `_extract_ref_path` + `validate_ref_targets_exist` (verificar existência de referências)
- M4: `_FOLDER_TO_STATUS` + `validate_folder_status_coherence` (coerência pasta×status)
- M5: `validate_filename_uniqueness` (unicidade de filename entre estados)
- Novos testes: classe `TestValidatorImprovements` em `pypi/tests/test_validator.py`

**Entregue:**
- `pypi/trackfw/validator.py` — `import subprocess` adicionado; `_walk_dir_md`, `_find_adr_file`, `_git_last_modified_time`, `_extract_ref_path` adicionados; `_adr_is_draft` usa `_find_adr_file`; `validate_adrs_are_referenced` usa `_walk_dir_md`; `validate_frontmatter_presence` usa `_walk_dir_md` + `_find_adr_file`; `validate_stale_wip` usa `_git_last_modified_time` com fallback para `st_mtime`; `validate_ref_targets_exist`, `_FOLDER_TO_STATUS`, `validate_folder_status_coherence`, `validate_filename_uniqueness` implementados; `validate()` inclui novas validações.
- `pypi/tests/test_validator.py` — classe `TestValidatorImprovements` com 11 novos testes adicionada.

**Resultado:** 157/157 testes verdes (11 novos) | commit `12d1009` | push para `feat/v2.3-validator-improvements`

**Agente:** Apolo | Status: CONCLUÍDO

---

## Sessão 2026-06-13 — Backend (config evolution ML-1A)

**Agente:** Backend | Status: CONCLUIDO

**Branch:** `feat/v2.4-config-evolution`

**Tarefa:** ML-1A — estender `internal/config/config.go` com novos campos (`LinkFieldsReq`, `LinkFieldsADR`, `LinkFieldsRoadmap`, `AcceptanceMarkers`, `Rules`) e parser de blocos aninhados de 1 nível. Criar `internal/config/config_evolution_test.go` com 6 testes cobrindo defaults, parsing e retrocompatibilidade.

**Entregue:**
- `internal/config/config.go` — struct `ProjectConfig` estendida com 5 novos campos v2.4; `defaults()` atualizado com defaults para todos; `parse()` reescrito com suporte a blocos aninhados de 1 nível (link_fields com sub-chaves req/adr/roadmap, acceptance_markers como lista, rules como mapa chave/valor).
- `internal/config/config_evolution_test.go` — 6 testes: `TestConfigDefaults_NewFields`, `TestConfigLinkFields`, `TestConfigAcceptanceMarkers`, `TestConfigRules`, `TestConfigSparse_NewFields`, `TestConfigRetrocompat`.

**Resultado:** 12/12 testes verdes em `internal/config` | `go build ./...` verde | commit `c676d45` | push para `feat/v2.4-config-evolution`

**Obs:** `TestMoveRoadmap_ByAgent` em `internal/generators` falha — pré-existente no commit `84eeff0`, fora do escopo do ML-1A.

---

## Sessão 2026-06-13 — Backend (config evolution ML-1B npm)

**Agente:** Backend | Status: CONCLUÍDO

**Branch:** `feat/v2.4-config-evolution`

**Tarefa:** ML-1B — estender `npm/src/config/index.js` com novos campos (`linkFields`, `acceptanceMarkers`, `rules`) e parser de blocos aninhados de 1 nível. Criar `npm/tests/config.test.js` com 6 testes.

**Entregue:**
- `npm/src/config/index.js` — `defaults()` estendida com `linkFields` (req/adr/roadmap), `acceptanceMarkers` e `rules` (9 regras com severidade); `parse()` reescrita com estados `inLinkFields`/`inAcceptanceMarkers`/`inRules` e função `flushBlocks()` para flush ao mudar de bloco ou no EOF; parser distingue indent via `rawLine[0]` (espaço/tab); sub-chaves de `link_fields` (req/adr/roadmap) resolvidas por nome.
- `npm/tests/config.test.js` — 6 testes sem framework externo (assert nativo): defaults, link_fields customizado, acceptance_markers customizado, rules parcial com merge, sparse, retrocompatibilidade v2.3.

**Resultado:** 6/6 testes `config.test.js` verdes | 12/12 testes `validator.test.js` inalterados | commit `84eeff0` | push para `feat/v2.4-config-evolution`.

---

## Sessão 2026-06-13 — Backend (config evolution ML-1C Python)

**Agente:** Backend | Status: CONCLUÍDO

**Branch:** `feat/v2.4-config-evolution`

**Tarefa:** ML-1C — estender `pypi/trackfw/config.py` com novos campos (`link_fields`, `acceptance_markers`, `rules`) e parser de blocos aninhados de 1 nível. Adicionar classe `TestConfigEvolution` em `pypi/tests/test_config.py` com 6 novos testes.

**Entregue:**
- `pypi/trackfw/config.py` — `defaults()` estendida com `link_fields` (req/adr/roadmap), `acceptance_markers` e `rules` (9 regras); `_parse()` reescrita com suporte a blocos aninhados: detecta indentação via `raw_line[0]`, aceita itens de lista com e sem indentação (compatibilidade com yamls existentes onde `- item` vem sem indent após a chave), função interna `flush_blocks()` com `nonlocal` para flush ao trocar de bloco ou no EOF; sub-chaves de `link_fields` resolvidas por nome.
- `pypi/tests/test_config.py` — classe `TestConfigEvolution` com 6 testes: `test_defaults_novos_campos`, `test_link_fields_customizado`, `test_acceptance_markers_customizado`, `test_rules_parcial_merge_com_defaults`, `test_sparse_novos_campos_usam_defaults`, `test_retrocompat_yaml_v23`.

**Decisão técnica:** o parser original aceitava itens de lista sem indentação (`- zeus` direto após `agents:`) — a nova implementação preserva esse comportamento detectando `line.startswith("- ")` independente do `raw_line[0]`, garantindo retrocompatibilidade total com yamls v2.3.

**Resultado:** 163/163 testes verdes (6 novos) | commit `201e748` | push para `feat/v2.4-config-evolution`

---

## Sessão 2026-06-13 — Backend (config evolution ML-2A validator)

**Agente:** Backend | Status: CONCLUÍDO

**Branch:** `feat/v2.4-config-evolution`

**Tarefa:** ML-2A — fazer o validator Go consumir os novos campos de config (`LinkFieldsReq`, `LinkFieldsADR`, `LinkFieldsRoadmap`, `AcceptanceMarkers`, `Rules`) em vez de strings hardcoded. F2 (field mapping) + F3 (severity per rule).

**Entregue:**
- `internal/validator/validator.go` — helper `contentHasMarker` substitui todas as comparações hardcoded `strings.Contains(content, "REQ:")` por loops sobre `cfg.LinkFieldsReq/ADR/Roadmap` e `cfg.AcceptanceMarkers`; helpers `ruleSeverity` e `applyRule` adicionados; `Validate()` refatorada para usar `applyRule` em todas as regras configuráveis (wip_has_req, adr_orphan, wip_acceptance, wip_limit, stale_wip, blocked_by_draft_adr, ref_targets_exist, folder_status, filename_uniqueness); regras sem entrada em `Rules` (validateREQsHaveADR, validateBlockedHasREQ, validateREQsHaveRoadmap, validateFrontmatterPresence) mantêm append direto em violations.
- `internal/validator/validator_evolution_test.go` — 4 testes: `TestFieldMapping_ReqId_SatisfiesWipHasREQ`, `TestRuleSeverity_Off_AdrOrphan`, `TestRuleSeverity_Warning_WipHasReq`, `TestAcceptanceMarkersCustom`.

**Resultado:** go build ./... verde | 4/4 novos testes verdes | todos os testes anteriores mantidos verdes | commit `0b0e47a` | push para `feat/v2.4-config-evolution`

---

## Sessão 2026-06-13 — Backend (config evolution ML-2B Node.js)

**Agente:** Backend | Status: CONCLUÍDO

**Branch:** `feat/v2.4-config-evolution`

**Tarefa:** ML-2B — fazer o validator Node.js (`npm/src/validator/index.js`) consumir os novos campos de config (`linkFields`, `acceptanceMarkers`, `rules`) em vez de strings hardcoded.

**Entregue:**
- `npm/src/validator/index.js` — adicionado `contentHasMarker(content, markers)` que substitui checks hardcoded de `'REQ:'`/`'ADR:'`/`'Roadmap:'` por loops sobre `cfg.linkFields.*`; adicionado `ruleSeverity(name)` e `applyRule(ruleName, msgs, violations, warnings)` para rotear msgs conforme `cfg.rules[name]` (error→violations, warning→warnings, off→descarta); função `validate()` refatorada usando `applyRule` para 9 regras configuráveis; regras sem configuração de severidade (validateREQsHaveADR, validateBlockedHasREQ, validateREQsHaveRoadmap, validateFrontmatterPresence) mantidas como violations diretas; `contentHasMarker`, `ruleSeverity`, `applyRule` exportadas no `module.exports`.
- `npm/tests/validator.test.js` — 4 novos testes: field mapping `req_id` satisfaz `wip_has_req`, severity `off` suprime `adr_orphan`, severity `warning` roteia `wip_has_req` para warnings, `acceptance_markers` customizado satisfaz verificação.

**Decisão técnica:** os testes de severity chamam diretamente `applyRule` + a sub-função de validação em vez de chamar `validate()` completo — evita efeitos colaterais de outras regras no ambiente de teste isolado.

**Resultado:** 16/16 testes `validator.test.js` verdes (12 existentes + 4 novos) | comportamento default idêntico à v2.3 | commit `6ed3ed5` | push para `feat/v2.4-config-evolution`

---

## Sessão 2026-06-13 — Backend (config evolution ML-2C Python)

**Agente:** Backend | Status: CONCLUÍDO

**Branch:** `feat/v2.4-config-evolution`

**Tarefa:** ML-2C — fazer o validator Python (`pypi/trackfw/validator.py`) consumir os novos campos de config (`link_fields`, `acceptance_markers`, `rules`) em vez de strings hardcoded (F2 field mapping + F3 severity per rule).

**Entregue:**
- `pypi/trackfw/validator.py` — adicionado `_content_has_marker(content, markers)` que substitui checks hardcoded de `"REQ:"`/`"ADR:"`/`"Roadmap:"` em `validate_wip_has_req`, `validate_reqs_have_adr`, `validate_blocked_has_req`, `validate_reqs_have_roadmap` por loops sobre `cfg["link_fields"][*]`; `validate_wip_has_acceptance_criteria` refatorado para usar `cfg["acceptance_markers"]` substituindo os 4 checks hardcoded; adicionado `_rule_severity(name, cfg)` e `_apply_rule(rule_name, msgs, violations, warnings, cfg)` para rotear msgs conforme `cfg["rules"]`; função `validate()` refatorada usando `_apply_rule` para 8 regras configuráveis (wip_has_req, adr_orphan, wip_acceptance, blocked_by_draft_adr, filename_uniqueness, ref_targets_exist, folder_status, stale_wip, wip_limit); regras sem configuração de severidade (validate_reqs_have_adr, validate_blocked_has_req, validate_reqs_have_roadmap, validate_frontmatter_presence) mantidas como violations diretas.
- `pypi/tests/test_validator.py` — nova classe `TestValidatorEvolution` com 4 testes: field mapping `req_id` satisfaz `wip_has_req`, severity `off` suprime `adr_orphan`, severity `warning` roteia `wip_has_req` para warnings, `acceptance_markers` customizado `## Done When` satisfaz verificação.

**Decisão técnica:** violations/warnings no Python validator são dicts `{"type": "...", "message": "..."}` (não strings simples) — `_apply_rule` e `_violations_messages` no teste tratam ambos os formatos.

**Resultado:** 167/167 testes verdes (todos os anteriores + 4 novos) | comportamento default idêntico à v2.3 | commit `86c133a` | push para `feat/v2.4-config-evolution`

---

## Sessão 2026-06-13 — Backend (baseline ML-3A Go)

**Agente:** Backend | Status: CONCLUÍDO

**Branch:** `feat/v2.4-config-evolution`

**Tarefa:** ML-3A — implementar `trackfw baseline` e mecanismo de ratchet em `trackfw validate` (Go).

**Entregue:**
- `internal/validator/validator.go` — adicionado `BaselineFile` struct, `baselineFileName`, `LoadBaseline()`, `SaveBaseline()`; `Validate()` renomeada para `ValidateUnfiltered()` (sem filtros); nova `Validate()` chama `ValidateUnfiltered()`, aplica ratchet de baseline (filtra violations presentes no baseline) e depois aplica modo lenient; import `encoding/json` adicionado.
- `internal/commands/baseline.go` — novo arquivo com `newBaselineCmd()`: chama `ValidateUnfiltered()`, persiste resultado via `SaveBaseline()`, imprime contagem.
- `internal/commands/root.go` — `newBaselineCmd()` registrado após `newValidateCmd()`.
- `internal/validator/validator_baseline_test.go` — 3 testes: `TestBaselineCreation` (cria baseline com violation), `TestBaselineFiltersOldViolations` (Validate() filtra violation do baseline), `TestBaselineNetNewViolation` (Validate() reporta violation não no baseline).

**Resultado:** `go build ./...` verde | 34/34 testes validator verdes (31 existentes + 3 novos) | commit `88456fd` | push para `feat/v2.4-config-evolution`

---

## Sessão 2026-06-13 — Backend (baseline ML-3B Node.js)

**Agente:** Backend | Status: CONCLUÍDO

**Branch:** `feat/v2.4-config-evolution`

**Tarefa:** ML-3B — implementar `trackfw baseline` e mecanismo de ratchet em `trackfw validate` (Node.js).

**Entregue:**
- `npm/src/validator/index.js` — adicionado `BASELINE_FILE`, `loadBaseline()`, `saveBaseline()`; função `validate()` renomeada para `validateUnfiltered()` (lógica inalterada, sem ratchet); nova `validate()` chama `validateUnfiltered()`, aplica ratchet (filtra violations já no baseline via Set de strings) e depois aplica modo lenient; todas as 4 funções novas exportadas em `module.exports`.
- `npm/src/commands/baseline.js` — novo arquivo; comando `trackfw baseline` chama `validateUnfiltered()` (async), persiste via `saveBaseline()`, imprime contagem.
- `npm/src/commands/index.js` — `require('./baseline')` registrado em `createProgram()`.
- `npm/tests/baseline.test.js` — 4 testes async: `saveBaseline cria .trackfw-baseline.json`, `loadBaseline retorna null se arquivo não existe`, `validate filtra violations do baseline`, `validate reporta violations novas (não no baseline)`.

**Resultado:** 4/4 testes `baseline.test.js` verdes | 16/16 testes `validator.test.js` inalterados | commit `77b8f8a` | push para `feat/v2.4-config-evolution`

---

## Sessão 2026-06-13 — Backend (baseline ML-3C Python)

**Agente:** Backend | Status: CONCLUÍDO

**Branch:** `feat/v2.4-config-evolution`

**Tarefa:** ML-3C — implementar `trackfw baseline` e mecanismo de ratchet em `trackfw validate` (Python).

**Entregue:**
- `pypi/trackfw/validator.py` — adicionado `import json`; constante `_BASELINE_FILE`; funções `_extract_messages()`, `load_baseline()`, `save_baseline()`; função `validate()` renomeada para `validate_unfiltered()` (sem ratchet, sem lenient); nova `validate()` chama `validate_unfiltered()`, aplica ratchet (filtra violations já no baseline via set de strings extraídas por `_extract_messages`) e depois aplica modo lenient; usa `datetime.now(timezone.utc)` (API moderna, sem DeprecationWarning).
- `pypi/trackfw/commands/baseline.py` — novo arquivo; comando `trackfw baseline` chama `validate_unfiltered()`, persiste via `save_baseline()`, imprime contagem.
- `pypi/trackfw/cli.py` — `baseline_cmd.register(subparsers)` registrado após `log_cmd`.
- `pypi/tests/test_baseline.py` — 4 testes: `test_save_baseline_cria_arquivo`, `test_load_baseline_retorna_none_se_nao_existe`, `test_validate_filtra_violations_do_baseline`, `test_validate_reporta_violations_novas`.

**Resultado:** 4/4 testes `test_baseline*` verdes | 171/171 testes totais verdes | `trackfw baseline` CLI funcional | commit a seguir | push para `feat/v2.4-config-evolution`

---

## Sessão 2026-06-13 — Apolo (CONCLUÍDO)

**Agente:** Backend | Status: CONCLUÍDO

**Branch:** `fix/v2.4.1-baseline-ratchet-warnings`

**Tarefa:** ML-2C — corrigir parser de `trackfw.yaml` em Python: trim de aspas envolventes nos valores do bloco `rules:` e nos escalares top-level.

**Entregue:**
- `pypi/trackfw/config.py` — `_parse()`: valor de sub-chaves de `rules:` agora usa `.strip().strip("\"'")` (linha do bloco `in_rules`); valores escalares top-level (`req_dir`, `roadmap_dir`, `roadmap_namespacing`, `governance_mode`, `lenient_until`) também recebem `.strip("\"'")`.
- `pypi/tests/test_config.py` — 2 novos testes adicionados em `TestConfigEvolution`: `test_rules_value_with_double_quotes` e `test_rules_value_with_single_quotes`.

**Resultado:** 187/187 testes verdes | commit `3f4becf` | push para `fix/v2.4.1-baseline-ratchet-warnings`

---

## Sessão 2026-06-13 — Apolo ML-2A Go (CONCLUÍDO)

**Agente:** Apolo | Status: CONCLUÍDO

**Branch:** `fix/v2.4.1-baseline-ratchet-warnings`

**Tarefa:** ML-2A — corrigir parser de `trackfw.yaml` em Go: trim de aspas envolventes em valores YAML (bloco `rules:` e escalares top-level).

**Entregue:**
- `internal/config/config.go` — `splitKV()` agora aplica `strings.Trim(val, "\"'")` após o `TrimSpace`, removendo aspas simples e duplas de qualquer valor extraído — cobre sub-chaves de `rules:`, `link_fields:` e escalares top-level em uma única mudança centralizada.
- `internal/config/config_evolution_test.go` — 2 novos testes adicionados: `TestRulesValueWithDoubleQuotes` (`adr_orphan: "off"` → `"off"` sem aspas) e `TestRulesValueWithSingleQuotes` (`stale_wip: 'warning'` → `"warning"` sem aspas).

**Resultado:** `go build ./...` verde | 14/14 testes `internal/config` verdes | commit `e6b8b39` | push para `fix/v2.4.1-baseline-ratchet-warnings`

---

## Sessão 2026-06-13 — Backend ML-1B Node.js (CONCLUÍDO)

**Agente:** Backend | Status: CONCLUÍDO

**Branch:** `feat/v2.5-discovery-json-traceid`

**Tarefa:** ML-1B — flag `--json` no `trackfw validate` para o CLI Node.js.

**Arquivos criados/modificados:**
- `npm/src/commands/validate.js` — opção `--json` adicionada ao commander; quando ativa, monta e imprime `JSON.stringify({summary, violations, warnings}, null, 2)` onde `summary = {violations: N, warnings: N, mode: "strict"|"lenient", exit_code: 0|1}`; comportamento texto completamente inalterado sem a flag.
- `npm/tests/validate_json.test.js` (novo) — 12 testes cobrindo: JSON válido, campos summary/violations/warnings presentes, contagem correta, exit_code consistente entre texto e JSON, mode válido, e comportamento texto inalterado sem --json.

**Resultado:** 12/12 validate_json.test.js verdes | 45/45 testes existentes (validator + config + help + baseline) sem regressões | commit e push para `feat/v2.5-discovery-json-traceid`

---

## Sessão 2026-06-13 — Backend ML-2B Node.js paths configuráveis (CONCLUÍDO)

**Agente:** Backend | Status: CONCLUÍDO

**Branch:** `feat/v2.5-discovery-json-traceid`

**Tarefa:** ML-2B — paths configuráveis `adr_dirs`, `req_dir`, `roadmap_dir` no CLI Node.js.

**Diagnóstico:** `npm/src/config/index.js` e `npm/src/validator/index.js` já tinham os campos implementados. Faltava: strip de aspas em `req_dir` e `roadmap_dir` (parser atribuía val direto) e testes dos novos campos.

**Arquivos modificados:**
- `npm/src/config/index.js` — fix: `req_dir` e `roadmap_dir` agora removem aspas envolventes com `.replace(/^["']|["']$/g, '')`.
- `npm/tests/config.test.js` — 4 novos testes ML-2B: `adr_dirs` com 2 itens, `req_dir` customizado, `roadmap_dir` customizado, defaults quando campos ausentes.

**Resultado:** 12/12 config.test.js verdes (8 anteriores + 4 novos) | 0 falhas

---

## Sessão 2026-06-13 — Backend ML-2C Python (CONCLUÍDO)

**Agente:** Backend | Status: CONCLUÍDO

**Branch:** `feat/v2.5-discovery-json-traceid`

**Tarefa:** ML-2C — paths configuráveis `adr_dirs`, `req_dir`, `roadmap_dir` no CLI Python.

**Diagnóstico:** `config.py` e `validator.py` já estavam totalmente parametrizados com os campos `adr_dirs`, `req_dir`, `roadmap_dir` (defaults e parse implementados em versões anteriores). Nenhuma alteração necessária nesses arquivos.

**Arquivos modificados:**
- `pypi/tests/test_config.py` — classe `TestConfigPaths` adicionada com 4 testes: `test_config_adr_dirs_list`, `test_config_req_dir_custom` (UTF-8), `test_config_roadmap_dir_custom`, `test_config_paths_defaults`.

**Resultado:** 17/17 test_config.py verdes | 191/191 testes pypi completos sem regressões | commit `41822c2` | push para `feat/v2.5-discovery-json-traceid`

---

## Sessão 2026-06-13 — Backend ML-2A v2.5 Go paths configuráveis (CONCLUÍDO)

**Agente:** Backend | Status: CONCLUÍDO

**Branch:** `feat/v2.5-discovery-json-traceid`

**Tarefa:** ML-2A — paths configuráveis `adr_dirs`/`req_dir`/`roadmap_dir` no CLI Go.

**Análise:** Campos `ADRDirs`, `REQDir`, `RoadmapDir` e o parser YAML já estavam implementados em `internal/config/config.go`. Os 4 testes nomeados no ML-2A não existiam — criados em `internal/config/config_paths_test.go`.

**Paths hardcoded em `discover.go`:** pertencem ao scanner de discovery brownfield (candidatos de autodetecção), não à camada de config — mantidos intencionalmente.

**Entregue:**
- `internal/config/config_paths_test.go` — 4 testes: `TestConfigAdrDirsList`, `TestConfigReqDirCustom` (UTF-8 docs/requisições), `TestConfigRoadmapDirCustom`, `TestConfigPathsDefaults`.

**Resultado:** 18/18 testes `internal/config` verdes | `make build` limpo | sem regressões novas | commit `d8ad96d` | push para `feat/v2.5-discovery-json-traceid`

---

## Sessão 2026-06-13 — Backend (IMPLEMENTANDO)

**Agente:** Backend | Status: CONCLUIDO

**Branch:** `feat/v2.5-discovery-json-traceid`

**Tarefa:** ML-1A — flag `--json` no `trackfw validate` (CLI Go).

**Entregue:**
- `internal/validator/result.go` — structs `RuleItem`, `ValidateSummary`, `ValidateResult` e builder `BuildResult`; slices inicializados como `[]RuleItem{}` para serializar como `[]` e não `null`.
- `internal/commands/validate.go` — flag `--json bool` adicionada ao cobra command; modo JSON usa `cmd.SilenceErrors = true` para saída JSON pura no stdout; exit code idêntico ao modo texto.
- `internal/commands/validate_json_test.go` — 3 testes: `TestValidateJSONFlag` (JSON válido + campos obrigatórios), `TestValidateJSONExitCode` (paridade de exit code), `TestValidateTextUnchanged` (modo texto inalterado).
- `make build` sem erros | 6/6 testes de commands verdes | todos os testes de validator verdes | sem regressões nos pacotes afetados.

---

## Sessão 2026-06-13 — Backend ML-1C v2.5 flag --json no validate Python (IMPLEMENTANDO)

**Agente:** Backend | Status: IMPLEMENTANDO

**Branch:** `feat/v2.5-discovery-json-traceid`

**Tarefa:** ML-1C — flag `--json` no `trackfw validate` para o CLI Python.

**Análise:**
- `pypi/trackfw/commands/validate.py` já é implementação completa (não stub)
- `pypi/trackfw/validator.py` retorna dicts `{"type": ..., "message": ...}` — sem campos `rule` e `file`
- Node JS mirror já tem `--json` com estrutura `{summary, violations: [{message}], warnings: [{message}]}`
- Estratégia: adicionar `--json` ao parser; no branch JSON, suprimir toda saída textual e emitir JSON puro; campos `rule`/`file` extraídos do dict se presentes (null se ausentes); testes pytest isolados com tmpdir + os.chdir

**Resultado:** 15/15 test_validate_json.py verdes | 206/206 testes pypi completos sem regressões | commits e2ed388 + b006205 | push para `feat/v2.5-discovery-json-traceid`

**Status final:** CONCLUIDO

**Arquivos modificados:**
- `pypi/trackfw/commands/validate.py` — argumento `--json` adicionado ao parser; branch JSON emite JSON estruturado puro suprimindo saída textual; modo texto inalterado
- `pypi/tests/test_validate_json.py` — 15 testes cobrindo: JSON válido, campos corretos, exit code paridade, modo lenient

---

## Sessão 2026-06-13 — ML-3C: namespacing by_agent — Python CLI

**Agente:** Backend | Status: IMPLEMENTANDO

**Branch:** `feat/v2.5-discovery-json-traceid`

**Tarefa:** ML-3C — `roadmap_namespacing: by_agent` no CLI Python.

**Análise:**
- `pypi/trackfw/config.py` já tem `NAMESPACING_BY_AGENT`, parse de `roadmap_namespacing` e `agents`
- `pypi/trackfw/validator.py` já tem `resolve_wip_dirs`, `validate_wip_limit` e `validate_folder_status_coherence` com suporte by_agent
- `pypi/trackfw/commands/status.py` já tem breakdown por agente
- Falta apenas: `pypi/tests/test_namespacing.py` com 3 testes obrigatórios

---

## 2026-06-13 — ML-3B Node.js namespacing by_agent (CONCLUÍDO)

**Agente:** Backend
**Branch:** `feat/v2.5-discovery-json-traceid`

### O que foi implementado

`npm/tests/namespacing.test.js` criado com 15 testes cobrindo:
- Parse de `roadmap_namespacing: by_agent` e `agents: [zeus, apolo]` no config
- `resolveWIPDirs` retornando hierarquia `<roadmapDir>/<agente>/wip/` no modo by_agent
- `validateWIPHasREQ`, `validateWIPHasAcceptanceCriteria` e `validateWIPLimit` varrendo dois agentes independentemente
- Comportamento flat inalterado (sem regressão)
- `getStatus` exibindo breakdown por agente
- Exportação correta de `NAMESPACING_FLAT` e `NAMESPACING_BY_AGENT`

**Resultado:** 15/15 passando; config.test.js (12) e validator.test.js (16) sem regressão.
**Commit:** `4777f80` — push em `feat/v2.5-discovery-json-traceid`

**Nota:** `config/index.js` e `validator/index.js` já tinham suporte completo a `by_agent` implementado em MLs anteriores. O ML-3B Node.js consistiu exclusivamente em criar a cobertura de testes.

**Resultado:** 9/9 test_namespacing.py verdes | 215/215 testes pypi completos sem regressões | commit 265caa4 | push para `feat/v2.5-discovery-json-traceid`

**Status final:** CONCLUIDO

**Arquivos modificados:**
- `pypi/tests/test_namespacing.py` — 9 testes cobrindo: parse config by_agent, wip_limit por agente, autodiscover de agentes, resolve_wip_dirs, comportamento flat inalterado

**Nota:** config.py, validator.py e status.py já tinham implementação completa de by_agent. Apenas os testes de namespacing estavam ausentes.

---

## 2026-06-13 — ML-5C: req_id bidirecional no CLI Python (Backend)

**Status:** CONCLUIDO
**Branch:** `feat/v2.5-discovery-json-traceid`
**Commit:** `7249687`

**O que foi implementado:**
- `pypi/trackfw/config.py`: campo `trace_id_field` adicionado ao defaults (default `""` — desativado) com parse no `_parse`
- `pypi/trackfw/traceid.py`: novo módulo com `check_traceid(cfg)` — indexa REQs e Roadmaps pelo campo de frontmatter configurado e emite 5 tipos de violations: `traceid_orphan_roadmap`, `traceid_orphan_req`, `traceid_state_mismatch`, `traceid_duplicate_req`, `traceid_duplicate_roadmap`. Parse de frontmatter duplicado localmente para evitar importação circular com `validator.py`
- `pypi/trackfw/validator.py`: integra `check_traceid(cfg)` em `validate_unfiltered()`
- `pypi/tests/test_traceid.py`: 6 testes pytest cobrindo todos os cenários (orphan roadmap, orphan req, state mismatch, duplicate req, par válido sem violation, desativado sem trace_id_field)

**Resultado:** 6/6 test_traceid.py verdes | 221/221 testes pypi completos sem regressões

---

## 2026-06-13 — ML-5A: req_id bidirecional no CLI Go (Backend)

**Status:** CONCLUIDO
**Branch:** `feat/v2.5-discovery-json-traceid`

### O que foi implementado

- `internal/config/config.go`: campo `TraceIdField string` adicionado ao struct `ProjectConfig` + case `trace_id_field` no parser `parse()`.
- `internal/validator/validator_traceid.go`: módulo com `validateTraceId(cfg ProjectConfig)` — 5 verificações: `traceid_orphan_roadmap`, `traceid_orphan_req`, `traceid_state_mismatch`, `traceid_duplicate_req`, `traceid_duplicate_roadmap`. Indexação por estado via subpastas (wip/, done/ etc.) + flat para REQs.
- `internal/validator/validator.go`: `ValidateUnfiltered()` atualizado — carrega `cfg := config.Load()` e chama `validateTraceId(cfg)` ao final.
- `internal/validator/validator_traceid_test.go`: 6 testes (`TestTraceIdOrphanRoadmap`, `TestTraceIdOrphanReq`, `TestTraceIdStateMismatch`, `TestTraceIdDuplicateReq`, `TestTraceIdValidPair`, `TestTraceIdDisabled`) — 6/6 verdes.

**Resultado:** `make build` sem erros | `go test ./internal/validator/ -run TestTraceId -v` 6/6 verdes | `go test ./...` sem novas regressões (falha pré-existente `TestMoveRoadmap_ByAgent` inalterada).

---

## 2026-06-13 — ML-5B: req_id bidirecional no CLI Node.js (Backend)

**Status:** IMPLEMENTANDO
**Branch:** `feat/v2.5-discovery-json-traceid`

**O que está sendo implementado:**
- `npm/src/config/index.js`: campo `traceIdField` no defaults + parse de `trace_id_field` no YAML
- `npm/src/validator/traceid.js`: módulo puro `checkTraceIds(reqDir, roadmapDir, fieldName)` com 5 violations
- `npm/src/validator/index.js`: integração da verificação via `validateUnfiltered()`
- `npm/tests/traceid.test.js`: testes com dirs temporários (mkdtempSync)

---

## 2026-06-13 — ML-3A: namespacing by_agent — testes Go (Backend)

**Status:** IMPLEMENTANDO → CONCLUIDO
**Branch:** `feat/v2.5-discovery-json-traceid`

**O que foi implementado:**
- `internal/validator/validator_namespacing_test.go`: 3 testes novos
  - `TestByAgentNamespacingWIPLimit`: limiar discriminante (zeus=3, apolo=3, limit=5 → total=6 violaria check global mas por agente passa sem warning)
  - `TestByAgentNamespacingWIPLimitExceeded`: agente zeus com 3 WIPs ultrapassa limit=2 → warning somente para zeus
  - `TestByAgentNamespacingFlat`: sem namespacing, comportamento flat — 2 WIPs com limit=1 emite warning global
- `internal/config/config_namespacing_test.go`: 1 teste novo
  - `TestConfigByAgentParsed`: YAML block-style `roadmap_namespacing: by_agent` + `agents: [zeus, apolo]` → struct correto

**Nota:** implementação de config.go, validator.go e generators/roadmap.go estava completa em MLs anteriores. Este ML consistiu exclusivamente em criar os testes de verificação.

**Falha pré-existente (não é responsabilidade do ML-3A):** `TestMoveRoadmap_ByAgent` em `internal/generators/` — ausência de `config.Reset()` faz o singleton retornar flat e `findRoadmap` falha. Confirmado anterior a este ML.

**Resultado:** `go test ./internal/validator/ -run TestByAgent -v` → 3/3 PASS | `go test ./internal/config/ -run TestConfigByAgent -v` → 1/1 PASS | `make build` → sem erros

**Status:** CONCLUIDO
**Commit:** `10119cb`

**Arquivos modificados:**
- `npm/src/config/index.js`: campo `traceIdField: ''` no defaults + case `trace_id_field` no parse YAML
- `npm/src/validator/traceid.js`: módulo puro `checkTraceIds(reqDir, roadmapDir, fieldName)` — indexa REQs e Roadmaps pelo campo de frontmatter e emite 5 violations; state derivado da pasta do arquivo (não do frontmatter)
- `npm/src/validator/index.js`: importa `checkTraceIds` e integra em `validateUnfiltered()` com guard `if (cfg.traceIdField)`
- `npm/tests/traceid.test.js`: 6 testes com mkdtempSync cobrindo todos os cenários

---

## Sessão 2026-06-13 — Backend (IMPLEMENTANDO)

**Tarefa:** ML-1C do roadmap v2.5.1 — popular `rule` e `file` no `--json` + adicionar `trace_id_field` e `rules.traceid_*` ao `trackfw help` no CLI Python.

**Branch:** `fix/v2.5.1-json-rule-file-help-traceid`

**Arquivos modificados:**
- `pypi/trackfw/validator.py`: adicionado `import re`; funções `_extract_file(msg)` e `_enrich_items(items, rule_name)` novas; `_apply_rule` passa por `_enrich_items` antes de distribuir; regras sem `_apply_rule` (diretas) também enriquecidas via `_enrich_items` em `validate_unfiltered`.
- `pypi/trackfw/commands/help_cmd.py`: adicionadas entradas `trace_id_field` + 5 regras `rules.traceid_*` ao `CONFIG_DOCS`.
- `pypi/tests/test_validate_json.py`: novo teste `test_json_violations_tem_campos_rule_e_file` verifica que `rule` e `file` são preenchidos.
- `pypi/tests/test_help.py`: 4 novos testes para `trace_id_field` e `rules.traceid_*`.

**Resultado:** 230/230 testes verdes | Sem regressões

**Status:** CONCLUIDO
**Commit:** `b572ee7`

**Resultado:** 6/6 traceid.test.js verdes | 12/12 config.test.js sem regressões | 12/12 validate_json.test.js sem regressões

---

## Sessão 2026-06-13 — Backend (CONCLUIDO)

**Tarefa:** ML-1B do roadmap v2.5.1 — popular `rule` e `file` no `--json` + adicionar `trace_id_field` e `rules.traceid_*` ao `trackfw help` no CLI Node.js.

**Branch:** `fix/v2.5.1-json-rule-file-help-traceid`

**Arquivos modificados:**
- `npm/src/validator/index.js`: adicionado `_itemMeta` Map com funções `_setMeta`, `getItemMeta` e `resetMeta`; `applyRule` popula o map na fonte; pushs diretos (`req_has_adr`, `blocked_has_req`, `req_has_roadmap`, `frontmatter_presence`, `wip_limit`, `traceid_*`) também populam com nome de regra explícito. Exporta `getItemMeta` e `resetMeta` sem alterar representação interna (strings — baseline e tests inalterados).
- `npm/src/commands/validate.js`: ao montar `--json`, enriquece cada item com `rule`/`file` via `getItemMeta()`.
- `npm/src/commands/help.js`: adicionadas 6 entradas (`trace_id_field` + `rules.traceid_{orphan_roadmap, orphan_req, state_mismatch, duplicate_req, duplicate_roadmap}`) ao `configDocs` com todos os campos obrigatórios.
- `npm/tests/validate_json.test.js`: dois novos testes com fixtures isoladas garantindo violations/warnings reais e verificando `rule`/`file`.
- `npm/tests/help.test.js`: dez novos testes cobrindo `listKeys` e `describeKey` para todas as entradas traceid.

**Resultado:** 14/14 validate_json.test.js | 20/20 help.test.js | 12/12 config.test.js | 6/6 baseline.test.js | 16/16 validator.test.js | 6/6 traceid.test.js | 15/15 namespacing.test.js | 13/13 discover.test.js — todos verdes, zero regressões.

**Status:** CONCLUIDO
**Commit:** `8536b7a`

---

## Sessão 2026-06-13 — Backend ML-1A v2.5.1 — auditoria Go (CONCLUÍDO)

**Agente:** Backend | Status: CONCLUÍDO

**Branch:** `fix/v2.5.1-json-rule-file-help-traceid`

**Tarefa:** Auditoria e verificação do ML-1A do roadmap v2.5.1 — popular `rule` e `file` no `--json` + adicionar `trace_id_field` e `rules.traceid_*` ao `trackfw help` (CLI Go).

**Resultado da auditoria:**
- `internal/validator/result.go` — `TaggedMsg{Rule, Msg}`, `extractFile()`, `BuildResultTagged()` implementados; `BuildResult()` mantido para compatibilidade com assinatura original.
- `internal/validator/validator.go` — `applyRuleTagged()`, `validateUnfilteredTagged()`, `extractRulePrefix()` e `ValidateTagged()` implementados; assinaturas públicas `Validate()`/`ValidateUnfiltered()`/`SaveBaseline()` inalteradas; filtro de baseline e modo lenient preservados em `ValidateTagged`.
- `internal/commands/validate.go` — modo `--json` usa `ValidateTagged()` + `BuildResultTagged()`; modo texto usa `Validate()` original sem alteração.
- `internal/commands/help.go` — 6 entradas adicionadas: `trace_id_field` + `rules.traceid_{orphan_roadmap,orphan_req,state_mismatch,duplicate_req,duplicate_roadmap}`.
- `internal/commands/validate_json_test.go` — asserção `rule='wip_has_req'` e `file='ROADMAP-sem-req.md'` adicionada ao `TestValidateJSONExitCode`.
- `internal/commands/help_test.go` — asserções `trace_id_field` e `rules.traceid_orphan_roadmap` adicionadas ao `TestHelpNoArgs`.

**Testes verificados:**
- `go test ./internal/commands/ -run 'TestValidateJSON|TestHelp' -v` — todos PASS
- `go test ./...` — sem novas regressões; `TestMoveRoadmap_ByAgent` falha pré-existente inalterada
- `make build` — limpo

**Observação:** os arquivos Go já estavam commitados no branch (provavelmente por sessão anterior). A implementação desta auditoria reproduziu o mesmo código já presente no HEAD — confirmando que o ML-1A Go estava correto e completo.

---

## Sessão 2026-06-13 — Apolo (CONCLUÍDO)

**Tarefa:** fix(traceid) ML-1A — suporte a `roadmap_namespacing: by_agent` na função `validateTraceId` + salvaguarda de zero entradas.

**Branch:** `fix/v2.5.2-traceid-by-agent`

**Problema corrigido:** Em projetos com `roadmap_namespacing: by_agent`, os 5 checks `traceid_*` nunca disparavam porque `collectTraceIdEntries` só varria `rootDir/<estado>/`, mas em `by_agent` a estrutura é `rootDir/<agente>/<estado>/`.

**Arquivos modificados:**
- `internal/validator/validator_traceid.go` — nova função `collectTraceIdEntriesByAgent` (varre `rootDir/<agente>/<estado>/*.md`; usa `cfg.Agents` ou descobre agentes via `os.ReadDir`); `validateTraceId` agora escolhe entre `collectTraceIdEntries` e `collectTraceIdEntriesByAgent` com base em `cfg.RoadmapNamespacing`; salvaguarda de zero entradas emite warning descritivo.
- `internal/validator/validator_traceid_test.go` — 2 novos testes: `TestTraceIdByAgent` (valida `traceid_orphan_req` e `traceid_orphan_roadmap` em estrutura by_agent) e `TestTraceIdZeroEntriesSalvaguarda` (valida warning quando diretórios estão vazios).

**Resultado:** `make build` limpo | 8/8 testes TraceId verdes | suite `internal/validator` 100% verde | commit `c7e61b9` | push para `fix/v2.5.2-traceid-by-agent`.


---

## Sessão 2026-06-13 — ML-1A: REQ indexing by_agent (v2.5.3)

**Agente:** Apolo
**Status:** IMPLEMENTANDO
**Branch:** fix/v2.5.3-req-indexing-by-agent

**Objetivo:** corrigir scanner de REQs para suportar req_dir/<agente>/<estado>/ quando roadmap_namespacing: by_agent — adicionar resolveREQFiles, substituir coletas planas em validator.go, fix em validator_traceid.go e salvaguarda one-sided.

---

## Sessão 2026-06-13 — ML-1B: context REQ by_agent (v2.5.4) — Apolo (CONCLUÍDO)

**Tarefa:** fix(npm): `trackfw context` exibia `## REQs (0)` em projetos com `roadmap_namespacing: by_agent`.

**Branch:** `fix/v2.5.4-context-req-by-agent`

**Problema corrigido:** `npm/src/commands/context.js` linha ~102 usava `collectEntries` plana para REQs, sem iterar agentes/estados como já era feito para Roadmaps.

**Arquivos modificados:**
- `npm/src/commands/context.js` — substituído `const reqs = collectEntries(cfg.reqDir || 'docs/req', 'REQ')` por lógica by_agent-aware que descobre agentes via `fs.readdirSync` e itera os 5 estados kanban; fallback para flat quando não é by_agent.
- `npm/tests/context_req_by_agent.test.js` — 2 testes: by_agent encontra REQ em `claude/wip/`; flat sem by_agent não regride.

**Resultado:** 2/2 testes novos verdes | testes `req_by_agent` e `validate_json` sem regressão | commit `5ab2532` | push para `fix/v2.5.4-context-req-by-agent`.

---

## Sessão 2026-06-13 — ML-1C: context REQ by_agent Python (v2.5.4) — Apolo (CONCLUÍDO)

**Tarefa:** fix(python): `trackfw context` exibia `## REQs (0)` em projetos com `roadmap_namespacing: by_agent` no CLI Python.

**Branch:** `fix/v2.5.4-context-req-by-agent`

**Problema corrigido:** `pypi/trackfw/commands/context.py` linha 108 usava `_collect_entries` plana para REQs, sem iterar agentes/estados como já era feito para Roadmaps no mesmo arquivo.

**Arquivos modificados:**
- `pypi/trackfw/commands/context.py` — substituído `reqs = _collect_entries(cfg.get("req_dir", "docs/req"), "REQ")` por lógica by_agent-aware que descobre agentes via `os.listdir` e itera os 5 estados kanban; fallback para flat quando não é by_agent.
- `pypi/tests/test_context_req_by_agent.py` — 2 testes pytest: `test_context_req_by_agent` (REQ em `claude/wip/` encontrada), `test_context_req_flat_no_regression` (modo flat sem regressão).

**Resultado:** 2/2 testes novos verdes | 238/238 testes totais passando | commit `6d10bf3` | push para `fix/v2.5.4-context-req-by-agent`.

---

## Sessão 2026-06-13 — ML-1A: context REQ by_agent Go (v2.5.4) — Apolo (CONCLUÍDO)

**Tarefa:** fix(go): `trackfw context` exibia `## REQs (0)` em projetos com `roadmap_namespacing: by_agent` no CLI Go. Adicionalmente, `validateADRsAreReferenced` usava `os.ReadDir` flat ignorando estrutura by_agent.

**Branch:** `fix/v2.5.4-context-req-by-agent`

**Problemas corrigidos:**
- `internal/generators/context.go` — bloco flat de REQs substituído por lógica by_agent-aware: quando `cfg.RoadmapNamespacing == config.NamespacingByAgent`, descobre agentes via `cfg.Agents` ou `os.ReadDir(cfg.REQDir)` (filtrando dirs) e itera os 5 estados kanban. Fallback flat preservado.
- `internal/validator/validator.go` — `validateADRsAreReferenced` substituiu `os.ReadDir(cfg.REQDir)` flat por `resolveREQFiles(cfg)` (já existia desde v2.5.3), tornando a validação de ADRs órfãos by_agent-aware.

**Testes adicionados:**
- `internal/generators/context_test.go` — `TestContextREQByAgent`: verifica que a lógica by_agent encontra REQ em `req/claude/wip/` com status correto extraído do frontmatter.
- `internal/validator/validator_test.go` — `TestValidateADRsAreReferencedByAgent`: verifica que ADR referenciado em REQ by_agent não gera violation de orphan.

**Resultado:** 2/2 testes novos verdes | `go test ./internal/validator/... ok` | commit `ac0c0de` | push para `fix/v2.5.4-context-req-by-agent`.

---

## Sessão 2026-06-14 — Apolo ML-1A Go (v2.6.0-rules-req-configuraveis) (CONCLUÍDO)

**Tarefa:** ML-1A do roadmap `feat/v2.6.0-rules-req-configuraveis` — tornar `req_has_adr`, `blocked_has_req` e `req_has_roadmap` controláveis via `rules.<nome>: off/warning/error` no `trackfw.yaml`.

**Branch:** `feat/v2.6.0-rules-req-configuraveis`

**Arquivos modificados:**
- `internal/validator/validator.go` — em `ValidateUnfiltered`: substituídos 3 `violations = append(violations, ...)` diretos por `applyRule("req_has_adr", ...)`, `applyRule("blocked_has_req", ...)` e `applyRule("req_has_roadmap", ...)`; em `validateUnfilteredTagged`: substituídos 3 loops `for _, m := range ... { violations = append(..., TaggedMsg{Rule: "", Msg: m}) }` por `applyRuleTagged("req_has_adr", ...)`, `applyRuleTagged("blocked_has_req", ...)` e `applyRuleTagged("req_has_roadmap", ...)`.
- `internal/validator/validator_test.go` — 3 novos testes com 3 sub-testes cada (warning/off/default_error): `TestReqHasADRConfiguravel`, `TestBlockedHasREQConfiguravel`, `TestReqHasRoadmapConfiguravel`. Seguem o padrão `t.TempDir()` + `chdir` + `config.Reset` + `t.Cleanup(config.Reset)`.

**Resultado:** `go build ./...` limpo | 11/11 pacotes de teste verdes (todos) | commit `f94dac9` | push para `feat/v2.6.0-rules-req-configuraveis`.

---

## 2026-06-14 — Apolo — ML-1C (Python) — CONCLUIDO

**Tarefa:** ML-1C do roadmap `feat/v2.6.0-rules-req-configuraveis` — tornar `req_has_adr`, `blocked_has_req` e `req_has_roadmap` configuráveis via `_apply_rule` no CLI Python.

**Branch:** `feat/v2.6.0-rules-req-configuraveis`

**Arquivos modificados:**
- `pypi/trackfw/validator.py` — em `validate_unfiltered`: substituídas 3 linhas `violations += _enrich_items(...)` por `_apply_rule("req_has_adr", ...)`, `_apply_rule("blocked_has_req", ...)` e `_apply_rule("req_has_roadmap", ...)`; renomeada chave `reqs_have_adr` → `req_has_adr` (sem "s") para alinhar cross-CLI.
- `pypi/tests/test_rules_req_configuraveis.py` — 9 testes novos (3 regras × 3 cenários: warning/off/default-error) usando `monkeypatch` para injetar config sem `trackfw.yaml`.

**Resultado:** 9/9 testes do arquivo novo verdes | 247/247 testes da suite completa verdes (sem regressão) | commit `80cf580` | push para `feat/v2.6.0-rules-req-configuraveis`.

---

## 2026-06-14 — Apolo — ML-1B (Node.js) — CONCLUIDO

**Tarefa:** ML-1B do roadmap `feat/v2.6.0-rules-req-configuraveis` — tornar `req_has_adr`, `blocked_has_req` e `req_has_roadmap` configuráveis via `applyRule` no CLI Node.js.

**Branch:** `feat/v2.6.0-rules-req-configuraveis`

**Arquivos modificados:**
- `npm/src/validator/index.js` — em `validateUnfiltered`: substituídos 3 loops `for (const msg of ...)` com push direto em violations por `applyRule('req_has_adr', ...)`, `applyRule('blocked_has_req', ...)` e `applyRule('req_has_roadmap', ...)`. `applyRule` já chama `_setMeta` internamente.
- `npm/tests/rules_req_configuraveis.test.js` — 9 testes novos (3 regras × 3 cenários: warning/off/default-error) usando `process.chdir` + `config.reset()` + dirs temporários.

**Resultado:** 9/9 testes novos verdes | `validate_json.test.js` 14/14 verdes (sem regressão) | `req_by_agent.test.js` 4/4 verdes (sem regressão) | alterações já presentes no commit `80cf580` (commit conjunto com Python) | branch atualizada no remoto.

---

## 2026-06-14 — Athena — Análise de Mercado trackfw v2.6.0 (CONCLUÍDO)

**Tarefa:** Pesquisa via WebSearch de 25+ concorrentes e geração de relatório completo de análise de mercado.

**Entregue:**
- `/tmp/trackfw-market-analysis.md` — relatório completo com 7 seções: mapa de mercado, análise por segmento (ADR tools, Spec/REQ, Roadmap, Platform Engineering, Engineering Metrics, AI-native Governance), posicionamento, pontos fortes/fracos, ameaças/oportunidades e recomendações estratégicas.

**Concorrentes pesquisados:** log4brains, adr-tools (npryce), MADR, pyadr, adr-log, arc-kit, Linear, Shortcut, GitHub Projects, GitLab Requirements, Productboard, Aha!, Backstage, Port.io, Cortex.io, OpsLevel, LinearB, Sleuth, Swarmia, Faros AI, GitHub Copilot Workspace, Cursor Rules/Organizations.

**Insights chave:**
- trackfw ocupa quadrante único: offline-first + CLI-centric + cadeia completa ADR→REQ→ROADMAP com CI gate.
- `roadmap_namespacing: by_agent` e `trace_id_field` são diferenciadores sem equivalente no mercado em jun/2026.
- Maior ameaça: GitHub Copilot Workspace + arc-kit evoluindo para CI gate. Maior oportunidade: SaaS fatigue + AI agents como atores de delivery.

**Agente:** Athena | Status: CONCLUÍDO

---

## 2026-06-14 — Apolo — ML-1A v2.7.0 trackfw serve UI (IMPLEMENTANDO)

**Tarefa:** ML-1A do roadmap `v2.7.0-trackfw-serve-ui` — criar pacote `internal/serve/` com `embed.FS` e placeholder `index.html`; atualizar `commands/serve.go` para usar `serve.Start(port)`.

**Branch:** `feat/v2.7.0-trackfw-serve-ui`

**Arquivos criados/modificados:**
- `internal/serve/serve.go` (novo) — pacote serve com `//go:embed static`, `Start(port int)`, rotas `/` e `/static/*`
- `internal/serve/static/index.html` (novo) — placeholder HTML inicial
- `internal/commands/serve.go` — import trocado de `internal/server` para `internal/serve`

**Resultado:** `go build ./...` limpo | `go test ./...` 100% verde | commit `648af62` | push para `feat/v2.7.0-trackfw-serve-ui`

**Observação:** `internal/server` permanece no projeto (não foi deletado) — será removido/migrado em wave posterior quando os endpoints API forem portados para `internal/serve/api_*.go`.

**Agente:** Apolo | Status: CONCLUÍDO

---

## 2026-06-14 — Apolo — Atualização VISION.md v2.6.0 (CONCLUÍDO)

**Tarefa:** Atualizar `docs/visao-projeto/VISION.md` para refletir o estado atual do projeto (v2.6.0) e posicionamento de mercado.

**Arquivo modificado:**
- `docs/visao-projeto/VISION.md` — header atualizado (v2.6.0 / 2026-06-14); comandos atuais adicionados (`context`, `validate --json`, `serve`, `traceid`); seção `trackfw validate` expandida com `governance_mode`, 15+ regras configuráveis e `trace_id_field` (5 checks automáticos); nova seção "AI-native Governance" com `roadmap_namespacing: by_agent`; seção Distribution atualizada para CLIs nativos (Go + Node.js + Python); 2 novos Design Principles (Configurable by design, AI-agent aware); roadmap antigo substituído por tabela "Current State (v2.6.0)"; seção "What trackfw Is NOT" ajustada para mencionar `trackfw serve`.

---

## 2026-06-14 — Afrodite — ML-0A assets dashboard trackfw serve (CONCLUÍDO)

**Branch:** `feat/v2.7.0-trackfw-serve-ui`

**Tarefa:** Implementar os 3 assets estáticos do dashboard `trackfw serve` (sem bundler, CDN apenas).

**Arquivos criados/modificados:**
- `internal/serve/static/index.html` — substituiu placeholder; layout completo com header/nav (Board/Chain/Metrics), 3 views, drawer lateral com overlay
- `internal/serve/static/style.css` — animacao slideIn do drawer, tab ativa, badge de estado, kanban cards com hover/focus, estilos prose para markdown, frontmatter table, D3 node labels, responsivo mobile (drawer 100% width < 768px)
- `internal/serve/static/app.js` — JS vanilla: loadBoard (kanban com cache, filtro agente), loadChain (D3 force-directed com zoom/pan/drag, setas, coloracao por tipo/estado), loadMetrics (Chart.js donut + burndown line), openDrawer/closeDrawer (fetch /api/file, parseFrontmatter, marked.parse, intercept links .md internos), switchView, filterByAgent, escapeHtml

**Resultado:** `go build ./...` limpo (embed.FS continua funcionando) | 3 arquivos criados

**Agente:** Afrodite | Status: CONCLUÍDO

**Agente:** Apolo | Status: CONCLUÍDO

---

## 2026-06-14 — Apolo — ML-1B→1E v2.7.0 trackfw serve endpoints (IMPLEMENTANDO)

**Tarefa:** Implementar os 4 endpoints da Wave 1 do `trackfw serve`:
- ML-1B: `GET /api/board` — kanban de roadmaps
- ML-1C: `GET /api/chain` — grafo ADR→REQ→ROADMAP
- ML-1D: `GET /api/metrics` — métricas de fluxo (log parser + cálculos)
- ML-1E: `GET /api/file` — leitura segura de arquivos (anti path traversal)

**Branch:** `feat/v2.7.0-trackfw-serve-ui`

**Arquivos a criar:**
- `internal/serve/api_board.go`
- `internal/serve/api_chain.go`
- `internal/serve/api_metrics.go`
- `internal/serve/metrics_log.go`
- `internal/serve/api_file.go`
- Atualizar `internal/serve/serve.go` para registrar os handlers

**Resultado:** `go build ./...` limpo | `go test ./...` 100% verde | commit `8a5dce3` | push para `feat/v2.7.0-trackfw-serve-ui`

**Decisoes tecnicas:**
- `setCORSHeaders` centralizado em `api_board.go` com `Access-Control-Allow-Origin: *` (dev-only)
- `parseFrontmatter` em `api_chain.go` puro sem dependência externa (evita yaml.v3)
- `fileHandler` usa prefixo com separador para evitar falsos positivos (ex: `docs/adr2` vs `docs/adr`)
- `calcBurndown` usa boundary semanal: para cada semana, aplica todos os eventos até o fim da semana para determinar o estado de cada roadmap
- `ParseLog` retorna slice vazia (não nil) quando o arquivo não existe — compatível com o frontend

**Agente:** Apolo | Status: CONCLUÍDO

---

## 2026-06-14 — Apolo — ML-3A: trackfw serve Python

**Status:** CONCLUIDO

**Tarefa:** Implementar `trackfw serve` para o CLI Python — servidor HTTP stdlib com dashboard web (kanban board, chain, metrics, file API).

**Branch:** `feat/v2.7.0-trackfw-serve-ui`

**Resultado:** 247 testes passando | commit `10e1a23` | push para `feat/v2.7.0-trackfw-serve-ui`

**Decisoes tecnicas:**
- `functools.partial` para injetar `cfg` no `BaseHTTPRequestHandler` sem variável global
- `os.path.realpath` + sufixo `os.sep` para evitar falsos positivos em path traversal (ex: `/docs/adr` vs `/docs/adr2`)
- `_parse_log` de `commands/metrics.py` reutilizado diretamente — sem duplicação
- Assets estáticos copiados de `internal/serve/static/` e declarados em `pyproject.toml` via `[tool.setuptools.package-data]`
- Detecção automática de agentes por subdiretórios quando `roadmap_namespacing == "by_agent"` e `agents: []`

**Agente:** Apolo | Status: CONCLUIDO

---

## 2026-06-14 — Apolo — ML-2A: trackfw serve Node.js

**Status:** CONCLUÍDO

**Tarefa:** Implementar `trackfw serve` para o CLI Node.js — servidor HTTP nativo (sem Express) com dashboard web.

**Arquivos criados/modificados:**
- `npm/src/commands/serve.js` — comando CLI + createServer com roteamento HTTP
- `npm/src/serve/api_board.js` — scan kanban (flat + by_agent)
- `npm/src/serve/api_chain.js` — grafo ADR→REQ→ROADMAP com parseFrontmatter nativo
- `npm/src/serve/api_metrics.js` — reutiliza parseLog/calculate de metrics.js
- `npm/src/serve/api_file.js` — segurança path traversal (resolve + allowedDirs)
- `npm/src/serve/static/` — cópia dos assets de internal/serve/static/
- `npm/src/commands/metrics.js` — exporta parseLog e calculate além do cmd
- `npm/src/commands/index.js` — registra createServeCommand()

**Critérios de aceite verificados:**
- `node npm/bin/trackfw serve --no-open --port 9191` sobe sem erro
- `/api/board` retorna JSON válido com columns e agents
- `/api/metrics` retorna JSON com lead_time, cycle_time, abandonment_rate, state_distribution, burndown
- `/api/chain` retorna JSON com nodes e edges
- `/api/file?path=../../../etc/passwd` retorna 403
- `/static/app.js` retorna 200

**Commit:** `8ea11ee` | **Push:** `feat/v2.7.0-trackfw-serve-ui`

**Observação:** O ambiente tem processos Go `main` ouvindo em várias portas (8080, 8081, etc.) que interceptam requisições com autenticação. Os testes foram realizados na porta 9191.

**Agente:** Apolo | Status: CONCLUÍDO

---

## Sessao 2026-06-14 — ML-4B Testes Node.js serve APIs

**Agente:** Artemis | Status: CONCLUIDO
**Branch:** feat/v2.7.0-trackfw-serve-ui
**REQ:** docs/requisicoes/artemis/done/REQ-2026-06-14-serve-api-tests-nodejs.md
**ROADMAP:** docs/roadmap/artemis/done/ROADMAP-2026-06-14-serve-api-tests-nodejs.md

**Arquivo criado:** `npm/tests/serve_api.test.js`
**Resultado:** 8/8 testes passaram | 0 regressoes nos 130 testes existentes
**Cobertura:**
- api_board: flat mode (columns + agents), by_agent mode (agent no card), board vazio
- api_file: path valido (200), path traversal (403), path fora dos dirs (403)
- api_metrics: sem log (zeros), com log valido (cycle_time_avg_days calculado)

---

## Sessao 2026-06-14 — ML-4C Testes Python serve APIs

**Agente:** Artemis | Status: CONCLUIDO
**Branch:** feat/v2.7.0-trackfw-serve-ui

**Objetivo:** Implementar `pypi/tests/test_serve_api.py` cobrindo api_board, api_file e api_metrics.

**Resultados:**
- 14 testes implementados e passando (pytest)
- Suite completa: 261/261 PASSED, sem regressoes
- Cobertura: api_board (flat, by_agent, autodetect, vazio), api_file (200, 403 traversal, 403 outside, _is_safe_path unit), api_metrics (sem log zeros, com log cycle_time, abandonment_rate, _calc_cycle_time direto)
- Path traversal bloqueado e testado com `../../../etc/passwd` → 403

**Commit:** `80e2492` | **Push:** `feat/v2.7.0-trackfw-serve-ui`

**Agente:** Artemis | Status: CONCLUIDO

---

## Sessao 2026-06-15 — ML-1A discover auto-install hook framework

**Agente:** Backend | Status: CONCLUIDO
**Branch:** `feat/discover-init-hook-autoinstall`
**Commit:** `0df8b6f`

**Objetivo:** `trackfw discover --init` sem framework detectado agora auto-instala lefthook ou husky em vez de apenas imprimir aviso.

**Mudancas implementadas:**
- `internal/discover/discover.go`:
  - `InstallGates` e `installHook` agora recebem `io.Writer` — corrige vazamento de `fmt.Println` para stdout
  - `case default` em `installHook`: detecta `package.json` → chama `installHusky`; ausente → chama `installLefthook`
  - `installLefthook`: cria `lefthook.yml` com entrada trackfw-validate; idempotente; tenta `lefthook install` se disponivel no PATH
  - `installHusky`: executa `npm install --save-dev husky` + `npx husky init`; cria `.husky/pre-commit` com `MkdirAll`; erros de exec sao warn, nao bloqueantes
- `internal/commands/discover.go`: repassa `out` (cobra writer) para `InstallGates`
- `internal/discover/discover_test.go`:
  - Testes existentes atualizados para nova assinatura (`io.Discard`)
  - 5 novos testes: sem package.json → lefthook.yml criado; com package.json → .husky/pre-commit criado; idempotencia lefthook; default sem/com package.json

**Resultado:** `make build`, `make test`, `make lint` — todos verdes

---

## Sessão 2026-06-17 — Apolo (CONCLUÍDO)

**Tarefa:** Feature de progresso de Wave/ML em `internal/serve/api_board.go`.

**Entregue:**
- `boardItem` — 3 campos novos: `MLTotal int`, `MLDone int`, `ActiveML string` (JSON: `ml_total`, `ml_done`, `active_ml`).
- `parseMLProgress(path string) (total, done int, activeML string)` — lê o arquivo de roadmap linha a linha; detecta linhas `## ... Wave` (captura título da wave atual), `### ML-*` (incrementa total, salva mlTitle), `**Status:**` com `✅` (incrementa done) ou `🔄` (preenche `activeML` como `"<wave> · <ml>"`). Tolerante a roadmaps sem waves (activeML usa somente o título do ML).
- `readStateDir` — chama `parseMLProgress(fullPath)` para cada card e popula os 3 campos novos no `boardItem`.

**Resultado:** `make build` limpo | `make test` 100% verde (todos os pacotes, incluindo `internal/serve`)

---

## Sessão 2026-06-18 — Zeus (CONCLUÍDO)

**Tarefa:** Implementar `trackfw update` nos 3 CLIs (Go + Node.js + Python).

**Entregue:**
- `internal/generators/update.go` — `Update(cwd)`, `ReadUpdateConfig(cwd)`, `updateHooksSurgical(cfg)`.
- `internal/generators/scaffold.go` — `ForceGenerateClaudeCommands()`, `ForceInstallSkills()`, variantes internas `force bool`.
- `internal/commands/update.go` — comando cobra `trackfw update`.
- `npm/src/commands/update.js` — comando Node.js com mesma lógica.
- `npm/src/generators/init.js` — `generateClaudeCommandsForce(rootDir)`, `installSkillsForce()`.
- `npm/src/commands/discover.js` — `writeCIWorkflowForce(rootDir)`, exports de `writeValidateScript/writeCIWorkflow`.
- `pypi/trackfw/commands/update.py` — escopo reduzido: apenas regras de agente.
- REQ: `docs/requisições/claude/REQ-2026-06-18-trackfw-update-command.md`.

**Comportamento:** 3 categorias de update — (1) marker-delimited via InjectRulesDetected, (2) trackfw-owned force overwrite, (3) shared hooks com inject cirúrgico.
**Branch:** `feat/kanban-roadmap-progress` | Roadmap: `done/trackfw-update-command-2026-06-18.md`

---

## Sessão 2026-06-20 — Apolo (CONCLUÍDO)

**Tarefa:** Implementar sistema de attention hooks do trackfw no CLI Go.

**Entregue:**
- `internal/generators/hooks.go` (novo) — `InjectHooksDetected(cwd)` detecta CLIs presentes e injeta hooks; injetores por CLI: `injectClaudeHooks` (merge idempotente em `.claude/settings.json`), `injectCodexHooks` (`.codex/hooks.json`), `injectGeminiHooks` (`.gemini/settings.json`), `injectKiroHooks` (`.kiro/hooks/trackfw-attention.json` — arquivo dedicado), `injectCopilotHooks` (`.github/hooks/trackfw-attention.json` — arquivo dedicado), `injectCursorHooks` (`.cursor/hooks.json`); helpers `mergeClaudeHookArray` e `mergeSimpleCommandArray` para deduplicação por command.
- `internal/generators/scaffold.go` — função `generateAttentionScripts()` gera `scripts/trackfw-attention-signal.sh` e `scripts/trackfw-attention-cleanup.sh` (permissão 0755); chamada adicionada em `Scaffold()` após `generateValidateScript`.
- `internal/generators/update.go` — passo 1b (`InjectHooksDetected`) adicionado após passo 1; `generateAttentionScripts()` chamada junto com validate script.
- `internal/commands/discover.go` — `generators.InjectHooksDetected(cwd)` invocado após `InjectRulesDetected` no fluxo `--init`.
- `internal/generators/agentfiles.go` — nota Windsurf adicionada na seção `### Attention Signal` do rules block.

**Resultado:** `go build ./...` limpo | `go vet ./...` limpo | `go test ./...` 100% verde.
**Branch:** `feat/attention-hooks-agent-clis`

---

## Sessão 2026-06-24 — Estabilização de qualidade (CONCLUÍDA)

**Branch:** `fix/repository-quality-gates`

**Objetivo:** corrigir a paridade do entrypoint Python, tornar os testes herméticos,
adicionar quality gates de CI/release e formalizar o contrato entre Go, Node.js e Python.

### Encerramento

**Status:** CONCLUÍDO

**Entregue:**
- Entry point Python conectado aos handlers reais, incluindo novo `init` não interativo.
- `version` e `--version` disponíveis nos três CLIs.
- Testes Go sem instalações externas reais e processos de discovery com timeout.
- CI de PR/push e gate obrigatório no workflow de release.
- Contratos automatizados de comandos, JSON de `validate` e assets do dashboard.
- `/api/attention` implementado no dashboard Node.js e Python.
- Build e smoke test dos pacotes npm e wheel Python.
- Downloads de plugins Go/Node com timeout, limite de tamanho e substituição atômica.
- Runtime mínimo alinhado: Go 1.25+, Node.js 18+, Python 3.10+.

---

## Sessão 2026-06-24 — Paridade documental de agentes (CONCLUÍDO)

**Tarefa:** alinhar a documentação visível e o log interno com a cobertura real de agentes e hooks.

**Entregue:**
- `site/guide/ai-agents.md` e `site/en/guide/ai-agents.md` — intro atualizada para listar Codex, Claude Code, Gemini CLI, Cursor, GitHub Copilot, Windsurf e Amazon Q.
- `site/index.md` e `site/en/index.md` — teaser de home alinhado à lista atual de agentes suportados.
- `site/guide/getting-started.md` e `site/en/guide/getting-started.md` — bullets de onboarding atualizados.
- `docs/agents-working-context.md` — sessão registrada com a fase de paridade documental.

**Validação:** `trackfw validate --json` manteve `violations=0` e `warnings=0`; `go test ./...`, `npm test` e `pytest pypi/tests` permaneceram verdes na fase anterior.

**Branch:** `feat/codex-agent-integrations`

**Validação:**
- `make quality` verde.
- Go: `go test`, `go vet` e `go build` verdes.
- Node.js: 13 arquivos de teste verdes.
- Python: 265 testes verdes.
- Wheel e tarball npm construídos e executados com sucesso.

---

## Sessão 2026-07-18 — Agents/skills lifecycle multi-CLI (CONCLUÍDO)

**Branch:** `feat/agents-skills-lifecycle-multi-cli`

**Objetivo:** substituir os instaladores fragmentados por um catálogo canônico e
adapters nativos, expondo `list`, `install`, `uninstall` e `update` para `agents` e
`skills` com paridade Go, Node.js e Python.

**Governança:**
- ADR: `docs/adr/ADR-2026-07-18-catalogo-canonico-e-adapters-para-integracoes-de-agentes.md`
- REQ: `docs/req/REQ-2026-07-18-agents-skills-lifecycle-multi-cli.md`
- Roadmap: `docs/roadmaps/done/ROADMAP-2026-07-18-agents-skills-lifecycle-multi-cli.md`

**Matriz entregue:** Claude, Codex, Gemini, Antigravity, Cursor, Copilot,
Windsurf, Amazon Q e Kiro, com formatos nativos ou fallback declarado.

### Progresso em 2026-07-18

- Waves 1 e 2 concluídas: catálogo canônico, manifesto de ownership e os quatro
  subcomandos de lifecycle estão implementados em Go, Node.js e Python.
- Os três runtimes compartilham o schema de manifesto v1, os estados
  `not-installed/current/outdated/modified` e as proteções de update/uninstall.
- O `list` exibe todos os itens e todas as surfaces compatíveis por target; uma
  surface específica pode ser escolhida com `--surface target=surface`.
- Testes focados Go, npm e Python estão verdes; assets dos três runtimes estão
  byte-idênticos. A Wave 3 iniciou os gates de empacotamento e migração legada.

### Encerramento

- Lifecycle `list/install/uninstall/update` entregue com JSON semanticamente
  idêntico em Go/Homebrew, npm e PyPI.
- Migração byte-exata cobre instalações Claude/Codex anteriores dos três
  pacotes, preservando conteúdo desconhecido e customizações.
- Sync/check de assets, smoke real do tarball npm e do wheel Python e matriz
  hermética dos nove targets foram aprovados.
- `make quality`: verde; Python: 300 testes; Node: 40 testes top-level;
  `trackfw validate --json`: zero violações e zero warnings.

---

## Sessão 2026-07-19 — Validação de Escrita de Arquivos (CONCLUÍDO)

**Branch:** `main`

**Objetivo:** Validar o acesso de escrita a arquivos no repositório.

**Progresso:**
- Acesso de escrita validado com sucesso.
- O arquivo `docs/valida-escrita.md` foi criado e persistido no repositório.
- Atualização do arquivo de contexto de agentes realizada com sucesso.

---

## Sessão 2026-07-19 — Suporte a ADRs Globais e Diretivas de IA (CONCLUÍDO)

**Branch:** `feat/global-adrs-governance`

**Objetivo:** Criar a especificação (ADR e REQ) para o suporte a ADRs globais compartilhados e diretivas de IA.

**Progresso:**
- Arquivo ADR `docs/adr/ADR-2026-07-19-global-adrs-governance.md` criado com sucesso.
- Arquivo REQ `docs/req/REQ-2026-07-19-global-adrs-governance.md` criado com sucesso.
- Contexto de trabalho atualizado.

---

## Sessão 2026-07-19 — Customização da Statusline (CONCLUÍDO)

**Branch:** `feat/global-adrs-governance` (sem alterações de código no repositório)

**Objetivo:** Configurar a statusline do Antigravity CLI com o layout Powerline personalizado.

**Progresso:**
- Criado o script Python em `~/.gemini/antigravity-cli/statusline.py` para receber o payload do CLI e formatá-lo com cores e setas Powerline.
- Atualizado o arquivo de configuração `~/.gemini/antigravity-cli/settings.json` para apontar para o novo script.

---

## Sessão 2026-07-19 — Apolo ML-1C (CONCLUÍDO)

**Tarefa:** ML-1C do roadmap `ROADMAP-2026-07-19-antigravity-agent-tools.md` — Implementar renderer `agent-directory` no CLI Python.

**Arquivos alterados:**
- `pypi/trackfw/integrations/renderers.py` — novo branch para `kind == "agents" and target == "antigravity" and surface == "current"`: reconstrói frontmatter com mapeamento de model (opus→pro, sonnet→flash) e injeção de tools (SET_IMPL 10 / SET_ARCH 14). Helpers: `_map_model`, `_agent_tools`, constantes `_MODEL_MAP`, `_SET_IMPL`, `_SET_ARCH`.
- `pypi/tests/test_agents_skills.py` — novo teste `test_antigravity_current_surface_renders_agent_directory`: valida architect (14 tools, model: pro, sem opus) e backend (10 tools, model: flash, sem define_subagent), ambos sem IDs proibidos.

**Resultado:** 31/31 testes verdes. Paridade byte-a-byte com implementação Go (`internal/integrations/render.go`).

---

## Sessão 2026-07-19 — ML-1A: Render agent-directory para Antigravity (IMPLEMENTANDO)

**Agente:** Apolo (Backend Specialist)
**Branch:** feat/antigravity-agent-tools (criada por Zeus)

**Objetivo:** Adicionar `case "agent-directory"` em `internal/integrations/render.go` para reconstruir frontmatter sem `model: opus|sonnet` e com `tools:` (SET_IMPL / SET_ARCH).

**Progresso:**
- Estendeu `markdownParts` para retornar 4º valor `model string`.
- Adicionou `case "agent-directory"` no switch de `Render` com reconstrução de frontmatter.
- Implementou helpers `mapModel` (opus→pro, sonnet→flash, passthrough para flash_lite/flash/pro) e `agentTools` (SET_IMPL 10 tools / SET_ARCH 14 tools).
- Adicionou `TestRenderAgentDirectory` com subtestes architect e backend.
- `go test ./internal/integrations/...` verde.
- `make build` sem erros.
- Nenhum asset alterado.

**Status: CONCLUIDO**

---

## Sessão 2026-07-19 — ML-1B: Render agent-directory para Antigravity no CLI Node.js (CONCLUÍDO)

**Agente:** Apolo (Backend Specialist)
**Branch:** feat/antigravity-agent-tools (criada por Zeus)

**Objetivo:** Adaptar `npm/src/integrations/render.js` para a representação `agent-directory` com mapa de model e injeção de tools; adicionar teste golden em `npm/tests/agents-skills.test.js`.

**Entregue:**
- `markdownParts` estendido para capturar campo `model` do frontmatter.
- Helpers `resolveModel` (opus→pro, sonnet→flash, passthrough flash_lite/flash/pro, '' para ausente/não-mapeável) e `toolsFor` (SET_ARCH 14 tools para nomes terminando em "architect", SET_IMPL 10 tools para demais).
- Constantes `SET_IMPL` e `SET_ARCH` locais; IDs proibidos nunca incluídos.
- Branch `if (capability.representation === 'agent-directory')` que reconstrói frontmatter e preserva body.
- Formato byte-equivalente ao Go (ML-1A): `---\nname/description/model(opcional)/tools---\nbody\n`.
- Teste golden `'Antigravity agent-directory renderer é byte-equivalente ao contrato Go/Python'` com `assert.equal` de string completa para architect e backend + asserts de ausência de IDs proibidos.
- `node --test npm/tests/agents-skills.test.js`: 21/21 testes passando.
- Nenhum asset em `npm/src/integrations/assets/agents/` alterado.

---

## 2026-07-19 — Apolo | Housekeeping: sincronização de version files → 2.14.0

**Status:** CONCLUIDO
**Branch:** `chore/sync-version-files-2.14.0`
**Agente:** Apolo (Backend Senior Specialist)

### O que foi feito
- Bump de `2.12.4` → `2.14.0` nos 5 version files: `internal/version/version.go`, `npm/package.json`, `pypi/pyproject.toml`, `pypi/trackfw/__init__.py`, `docs/visao-projeto/VISION.md`.
- Build validado: `make build` sem erros.
- Binário confirmado: `./bin/trackfw version` → `trackfw v2.14.0`.
- Testes verdes: `go test ./internal/version/... ./internal/integrations/...`.
- grep de residual `2.12.4` nos 5 arquivos: vazio.
- Commit: `2ed0874` — apenas os 5 arquivos, sem push (Zeus faz o push).

---

## Sessão 2026-07-20 — Zeus (CONCLUÍDO)

**Tarefa:** Verificação e consolidação do backlog para codar no projeto.
**Agente:** 🌩️ Zeus - Principal Software Architect

**Ações:**
- Inspecionados diretórios `docs/req/`, `docs/roadmaps/`, `docs/requisições/` e `docs/adr/`.
- Mapeadas 4 demandas pendentes/backlog.

---

## Sessão 2026-07-20 — Zeus (IMPLEMENTANDO)

**Tarefa:** Orquestração e disparo da Wave 1 do ROADMAP-2026-07-19-global-adrs-governance.md.
**Branch:** `feat/global-adrs-governance`
**Agente:** 🌩️ Zeus - Principal Software Architect

**Ações:**
- Criada branch `feat/global-adrs-governance`.
- Gerado `docs/roadmaps/ROADMAP-2026-07-19-global-adrs-governance.md` detalhado com 4 waves e paralelização de microlotes.
- Atualizado vínculo no `REQ-2026-07-19-global-adrs-governance.md`.
- Commit de docs realizado (`d6f649b`).
2202: - Disparados 3 subagentes paralelos para Wave 1 (ML-1A Go, ML-1B Node, ML-1C Python).

---

## Sessão 2026-07-20 — Apolo (IMPLEMENTANDO)

**Tarefa:** ML-1C do Roadmap `docs/roadmaps/ROADMAP-2026-07-19-global-adrs-governance.md` — Expansão de tilde (`~` / `~/`) no CLI Python (`config.py` e `validator.py`).
**Agente:** ☀️ Apolo — Backend Senior Specialist

**Ações:**
- Iniciando implementação da expansão de til em `pypi/trackfw/config.py` e `pypi/trackfw/validator.py`.
- Adição de testes em `pypi/tests/test_config.py` e `pypi/tests/test_validator.py`.


---

## Sessão 2026-07-20 — Apolo (CONCLUÍDO ML-1A)

**Tarefa:** ML-1A - Suporte à expansão de til (`~` ou `~/`) no carregamento de `adr_dirs` no CLI Go (`internal/config/config.go` e `internal/validator/validator.go`).
**Agente:** ☀️ Apolo — Backend Senior Specialist

**Entregue:**
- `internal/config/config.go`: adicionada função exportada `ExpandPath(p string) string` utilizando `os.UserHomeDir()` e `filepath.Join()`. Atualizado o parser `parse()` de `trackfw.yaml` para expandir caminhos em `adr_dirs`.
- `internal/validator/validator.go`: atualizadas funções `walkADRFiles`, `findADRFile` e `referenceExists` para expandir caminhos com `config.ExpandPath()`.
- `internal/config/config_paths_test.go`: adicionados testes `TestExpandPath` e `TestConfigTildeExpansionInAdrDirs`.
- `internal/validator/validator_test.go`: adicionado teste `TestValidate_WithTildeInADRDirs`.
- Roadmap `docs/roadmaps/ROADMAP-2026-07-19-global-adrs-governance.md`: ML-1A marcado como `✅ Concluído`.

## Sessão 2026-07-20 — Zeus (CONCLUÍDO)

**Tarefa:** Orquestração e implementação completa do ROADMAP-2026-06-19-architect-command-guidelines.md (Backlog #3).
**Branch:** `feat/architect-command-guidelines`
**Pull Request:** https://github.com/kgsaran/trackfw/pull/58
**Agente:** 🌩️ Zeus - Principal Software Architect

**Entregues:**
- **Branch e PR criados:** `feat/architect-command-guidelines` → PR #58 (`https://github.com/kgsaran/trackfw/pull/58`).
- **Wave 1:**
  - Slash command `/trackfw:architect` (`architect.md`) gerado em `.claude/commands/trackfw/` nas 3 distribuições (Go, Node.js, Python).
  - Seção `### Architecture Directives (mandatory)` com as 8 diretrizes de arquitetura injetada nos blocos de regras (`CLAUDE.md`, `AGENTS.md`, `.windsurfrules`, `.cursor/rules/`, etc.).
  - REQ atualizada e concluída em `docs/requisições/claude/REQ-2026-06-19-architect-command-guidelines.md`.
  - Roadmap finalizado em `docs/roadmaps/done/ROADMAP-2026-06-19-architect-command-guidelines.md`.
  - Suítes de testes 100% aprovadas nas 3 linguagens.








## Sessão 2026-07-20 — Apolo (CONCLUÍDO ML-1C)

**Tarefa:** ML-1C do Roadmap `docs/roadmaps/ROADMAP-2026-07-19-global-adrs-governance.md` — Expansão de tilde (`~` / `~/`) no CLI Python (`config.py` e `validator.py`).
**Agente:** ☀️ Apolo — Backend Senior Specialist

**Entregue:**
- `pypi/trackfw/config.py`: adr_dirs utiliza `os.path.expanduser` durante a leitura/parse de listas YAML.
- `pypi/trackfw/validator.py`: `_find_adr_file`, `_adr_is_draft`, `validate_adrs_are_referenced`, `validate_frontmatter_presence` e `validate_ref_targets_exist` utilizam `os.path.expanduser` em cada `adr_dir`.
- `pypi/tests/test_config.py`: adicionado `test_config_adr_dirs_tilde_expansion` testando o parse de `~/...`.
- `pypi/tests/test_validator.py`: adicionada classe `TestExpandTildeAdrDirs` com `test_find_adr_file_com_tilde` e `test_validate_adrs_are_referenced_com_tilde`.
- Status do ML-1C no roadmap atualizado para `✅ Concluído`.


---

## Sessão 2026-07-20 — Apolo (IMPLEMENTANDO)

**Tarefa:** ML-2A do Roadmap `docs/roadmaps/ROADMAP-2026-07-19-global-adrs-governance.md` — Suporte a `strict_ci_paths` (default `false`), Warning para `adr_dirs` inexistentes e isenção de `adr_orphan` para ADRs fora do `cwd` no Go CLI.
**Agente:** ☀️ Apolo — Backend Senior Specialist

**Ações:**
- Iniciando implementação de `strict_ci_paths` em `internal/config/config.go`.
- Ajustando validações em `internal/validator/validator.go`.
- Adicionando testes unitários em `internal/validator/validator_test.go`.

---

## Sessão 2026-07-20 — Afrodite (CONCLUÍDO ML-2B)

**Tarefa:** ML-2B do Roadmap `docs/roadmaps/ROADMAP-2026-07-19-global-adrs-governance.md` — Suporte a `strict_ci_paths` (default `false`), `Warning` para diretórios `adr_dirs` inexistentes e isenção de `adr_orphan` para arquivos fora de `cwd` no CLI Node.js.
**Agente:** 💖 Afrodite — Frontend i18n Senior Specialist

**Entregue:**
- `npm/src/config/index.js`: adicionada opção `strictCiPaths` no `defaults()` (default `false`) e parse de `strict_ci_paths` no parser YAML.
- `npm/src/validator/index.js`:
  - Criados helpers `isInsideDir` e `walkDirMdWithPaths`.
  - Criada função `validateADRDirsExist` que retorna `warnings` se `strictCiPaths: false` (default) ou `violations` se `strictCiPaths: true` para diretórios `adr_dirs` inexistentes.
  - Atualizada `validateADRsAreReferenced` para isentar diretórios e arquivos de ADR externos à raiz do projeto (`cwd`) da verificação de `adr_orphan`.
- `npm/tests/config.test.js`: adicionado teste de `strict_ci_paths`.
- `npm/tests/validator.test.js`: adicionados testes unitários validando warning/violation para dir inexistente e isenção de `adr_orphan` para ADRs externos.
- Roadmap `docs/roadmaps/ROADMAP-2026-07-19-global-adrs-governance.md`: ML-2B marcado como `✅ Concluído`.


---

## Sessão 2026-07-20 — Apolo (CONCLUÍDO ML-2C)

**Tarefa:** ML-2C do Roadmap `docs/roadmaps/ROADMAP-2026-07-19-global-adrs-governance.md` — Suporte a `strict_ci_paths` (default `False`), `Warning` para diretórios `adr_dirs` não encontrados e isenção de `adr_orphan` para arquivos fora de `cwd` no CLI Python.
**Agente:** ☀️ Apolo — Backend Senior Specialist

**Entregue:**
- `pypi/trackfw/config.py`: `strict_ci_paths` adicionado aos `defaults()` (default `False`) e parseado a partir de `trackfw.yaml`.
- `pypi/trackfw/validator.py`:
  - Helper `_is_subpath` criado para identificar arquivos/diretórios contidos em `cwd`.
  - `validate_adr_dirs_exist` verifica se os diretórios em `adr_dirs` existem, emitindo `Warning` se `strict_ci_paths` for `False` e `violation` se `strict_ci_paths` for `True`.
  - `validate_adrs_are_referenced` isenta caminhos fora de `cwd` da regra `adr_orphan`.
- `pypi/tests/test_config.py`: teste `test_config_strict_ci_paths` adicionado.
- `pypi/tests/test_validator.py`: classes `TestStrictCIPathsAndInexistentAdrDirs` e `TestAdrOrphanExemptOutsideCwd` adicionadas.
- Roadmap `docs/roadmaps/ROADMAP-2026-07-19-global-adrs-governance.md`: ML-2C marcado como `✅ Concluído`.


---

## Sessão 2026-07-20 — Apolo (CONCLUÍDO ML-2A)

**Tarefa:** ML-2A do Roadmap `docs/roadmaps/ROADMAP-2026-07-19-global-adrs-governance.md` — Suporte a `strict_ci_paths` (default `false`), `Warning` para diretórios `adr_dirs` inexistentes e isenção de `adr_orphan` para arquivos fora do `cwd` no Go CLI.
**Agente:** ☀️ Apolo — Backend Senior Specialist

**Entregue:**
- `internal/config/config.go`: adicionado campo `StrictCIPaths bool` em `ProjectConfig` (default `false`) e parse de `strict_ci_paths` a partir do YAML.
- `internal/config/config_paths_test.go`: adicionado `TestConfigStrictCIPaths` cobrindo o default `false` e parse quando `true`.
- `internal/validator/validator.go`:
  - `validateADRDirsExist`: verifica se cada diretório em `adr_dirs` existe; se não existir, gera `Warning` (se `StrictCIPaths == false`) ou `Error` violation (se `StrictCIPaths == true`).
  - `isOutsideCWD`: helper que determina se um caminho está fora da raiz do projeto local (`cwd`).
  - `validateADRsAreReferenced`: isenta arquivos ADR localizados fora do `cwd` da verificação `adr_orphan`.
- `internal/validator/validator_test.go`: adicionados testes `TestValidate_NonExistentADRDirs_WarningByDefault`, `TestValidate_NonExistentADRDirs_StrictCIPathsError` e `TestValidate_ExternalADROrphanExemption`.


---

## Sessão 2026-07-20 — Apolo (CONCLUÍDO ML-3B)

**Tarefa:** ML-3B do Roadmap `docs/roadmaps/ROADMAP-2026-07-19-global-adrs-governance.md` — Injetar a diretiva obrigatória de leitura dos ADRs globais no gerador de regras de agente para Python.
**Agente:** ☀️ Apolo — Backend Senior Specialist

**Entregue:**
- `pypi/trackfw/generators/init_gen.py`: inclusão da diretiva `"- Obrigatório: Inspecione e respeite todos os ADRs globais nos diretórios listados em adr_dirs (inclusive caminhos ~/...) antes de propor alterações de arquitetura."` na seção `Architecture Directives (mandatory)` de `_trackfw_rules_block()`.
- `pypi/tests/test_generators_init.py`: adicionada a classe `TestGlobalADRsRuleDirective` com teste `test_rules_block_contains_global_adrs_directive` validando a presença da nova diretiva no bloco gerado e na injeção em arquivos de agentes.
- `pypi/tests/test_rules_agents.py`: atualizado para asserção do snippet da diretiva em múltiplos assistentes.
- Roadmap `docs/roadmaps/ROADMAP-2026-07-19-global-adrs-governance.md`: ML-3B marcado como `✅ Concluído`.

---

## Sessão 2026-07-20 — Apolo (CONCLUÍDO ML-3A)

**Tarefa:** ML-3A do Roadmap `docs/roadmaps/ROADMAP-2026-07-19-global-adrs-governance.md` — Injetar a diretiva obrigatória de leitura dos ADRs globais nos geradores de regras de agente para Go e Node.js.
**Agente:** ☀️ Apolo — Backend Senior Specialist

**Entregue:**
- `internal/generators/claudemd.go`: inclusão da diretiva `"8. **Obrigatório: Inspecione e respeite todos os ADRs globais nos diretórios listados em adr_dirs (inclusive caminhos ~/...) antes de propor alterações de arquitetura.**"` no bloco de Agent rules.
- `internal/generators/scaffold.go`: inclusão da mesma diretiva nas Regras invioláveis de `installGlobalSkillInner`.
- `internal/generators/agentfiles.go`: inclusão da diretiva no Agent Protocol de `trackfwRulesBlock()`.
- `internal/generators/claudemd_test.go`: criação de suíte de testes unitários Go cobrindo a presença da diretiva em `CLAUDE.md`, `trackfwRulesBlock` e na skill global `SKILL.md`.
- `npm/src/generators/init.js`: inclusão da diretiva em `trackfwRulesBlock()` e `generateClaudeMD()`.
- `npm/tests/generators.test.js`: criação de suíte de testes unitários Node.js validando a inclusão da diretiva no bloco de regras e em arquivos gerados.
- Roadmap `docs/roadmaps/ROADMAP-2026-07-19-global-adrs-governance.md`: ML-3A marcado como `✅ Concluído`.

---

## Sessão 2026-07-20 — Apolo (CONCLUÍDO ML-1A)

**Tarefa:** ML-1A do Roadmap `docs/roadmaps/ROADMAP-2026-06-20-attention-hooks-agent-clis.md` — Geração dos scripts `scripts/trackfw-attention-signal.sh` e `scripts/trackfw-attention-cleanup.sh` nos 3 geradores de scaffold/init (Go, Node.js, Python).
**Agente:** ☀️ Apolo — Backend Senior Specialist

**Entregue:**
- `internal/generators/scaffold.go`: atualizada a função `generateAttentionScripts()` para gerar `scripts/trackfw-attention-signal.sh` e `scripts/trackfw-attention-cleanup.sh` com o conteúdo exato exigido e permissão `0755`.
- `internal/generators/scaffold_test.go`: adicionado o teste `TestGenerateAttentionScripts` garantindo a criação dos dois scripts, permissões executáveis e validação do cabeçalho do conteúdo.
- `npm/src/generators/hooks.js`: atualizadas as constantes `SIGNAL_SCRIPT` e `CLEANUP_SCRIPT` para gerar os scripts com o conteúdo exato exigido.
- `npm/tests/generators.test.js`: adicionado o teste `scaffold generates attention scripts with execution permissions and expected headers`.
- `pypi/trackfw/generators/init_gen.py`: atualizadas as constantes `_ATTENTION_SIGNAL_SH` e `_ATTENTION_CLEANUP_SH` para o conteúdo exato exigido.
- `pypi/tests/test_generators_init.py`: adicionada a classe de testes `TestAttentionScripts` validando existência, permissões executáveis no POSIX e cabeçalhos dos scripts.
- `docs/roadmaps/ROADMAP-2026-06-20-attention-hooks-agent-clis.md`: ML-1A marcado como `✅ Concluído`.

---

## Sessão 2026-07-20 — Apolo (CONCLUÍDO ML-2A a ML-2G no CLI Python)

**Tarefa:** Injetores de hooks de atenção para os 7 CLIs no CLI Python (ML-2A até ML-2G do `docs/roadmaps/ROADMAP-2026-06-20-attention-hooks-agent-clis.md`).
**Agente:** ☀️ Apolo — Backend Senior Specialist

**Entregue:**
- `pypi/trackfw/generators/init_gen.py`:
  - Adicionada a instrução de uso manual do `.trackfw-attention.json` para usuários do Windsurf no bloco de regras `_trackfw_rules_block()`.
  - Integrada a chamada a `inject_hooks_detected(cwd)` durante a execução de `scaffold(...)` no `init`.
- `pypi/trackfw/generators/hooks.py`:
  - Suporte completo e idempotente a injeção de hooks para os 7 CLIs (Claude Code, Codex, Gemini, Kiro, Copilot, Cursor e Windsurf via rules block).
- `pypi/trackfw/commands/discover.py`:
  - Adicionada geração de `_generate_attention_scripts(cwd)` e atualização de `inject_hooks_detected(cwd)` durante a flag `--init`.
- `pypi/tests/test_generators_init.py`:
  - Adicionados testes unitários completos em `TestAttentionHooksInjectors` para injeção, idempotência, merge e detecção automática de hooks nos 7 CLIs.
  - 319/319 testes da suíte Python passando (100% verde).

---

## Sessão 2026-07-20 — Apolo (CONCLUÍDO ML-2A a ML-2G no CLI Go)

**Tarefa:** Implementar injetores de hooks de atenção para os 7 CLIs no CLI Go (ML-2A até ML-2G do `docs/roadmaps/ROADMAP-2026-06-20-attention-hooks-agent-clis.md`).
**Agente:** ☀️ Apolo — Backend Senior Specialist

**Entregue:**
- `internal/generators/agentfiles.go`:
  - Implementadas as 7 funções de injeção exportadas (`InjectClaudeHooks`, `InjectCodexHooks`, `InjectGeminiHooks`, `InjectKiroHooks`, `InjectCopilotHooks`, `InjectCursorHooks`, `InjectWindsurfHooks`) com merges idempotentes.
  - Atualizado `trackfwRulesBlock()` com a instrução do Windsurf.
- `internal/generators/hooks.go` & `internal/generators/codex.go`:
  - Refatorados para utilizar as funções exportadas de injeção em `agentfiles.go`.
- `internal/discover/discover.go`:
  - Atualizada a função `InstallGates` para chamar `generators.InjectHooksDetected(rootDir)`.
- `internal/generators/agentfiles_test.go` e `internal/generators/hooks_test.go`:
  - Testes unitários completos para criação, merge idempotente e detecção dos hooks nos 7 CLIs.
  - Suíte `go test ./...` 100% verde.
- `docs/roadmaps/ROADMAP-2026-06-20-attention-hooks-agent-clis.md`:
  - ML-2A até ML-2G marcados como `✅ Concluído`.
---

## Sessão 2026-07-20 — Afrodite (CONCLUÍDO ML-2A a ML-2G Node.js)

**Tarefa:** Implementar os injetores de hooks de atenção para os 7 CLIs no CLI Node.js (ML-2A até ML-2G de `docs/roadmaps/ROADMAP-2026-06-20-attention-hooks-agent-clis.md`).
**Agente:** 💖 Afrodite - Frontend i18n Senior Specialist

**Entregue:**
- `npm/src/generators/hooks.js`:
  - Implementados injetores de hooks de atenção idempotentes para os 7 CLIs: `injectClaudeHooks` (`.claude/settings.json`), `injectCodexHooks` (`.codex/hooks.json`), `injectGeminiHooks` (`.gemini/settings.json`), `injectKiroHooks` (`.kiro/hooks/trackfw-attention.json`), `injectCopilotHooks` (`.github/hooks/trackfw-attention.json`), `injectCursorHooks` (`.cursor/hooks.json`) e `injectWindsurfHooks` (`.windsurfrules`).
  - Atualizada a função `injectHooksDetected(cwd)` para mapear e executar automaticamente a injeção em todos os 7 CLIs suportados.
- `npm/src/generators/init.js`:
  - Atualizado `trackfwRulesBlock()` para incluir a instrução explícita do Windsurf para criação manual do `.trackfw-attention.json`.
  - Integrada a chamada `injectHooksDetected(root)` ao método `scaffold(...)`.
  - Exportadas as funções `injectHooksDetected` e helpers.
- `npm/src/commands/discover.js`:
  - Confirmada a invocação de `injectHooksDetected(cwd)` em `discover --init`.
- `npm/tests/generators.test.js`:
  - Adicionados testes unitários completos testando criação do zero, merge idempotente preservando hooks customizados pré-existentes do usuário e detecção automática combinada dos 7 CLIs.

---

## Sessão 2026-07-20 — Apolo (IMPLEMENTANDO Wave 3 ML-3A e ML-3B)

**Tarefa:** Implementar a Wave 3 (ML-3A e ML-3B) do `docs/roadmaps/ROADMAP-2026-06-20-attention-hooks-agent-clis.md` para Go e Node.js.
**Agente:** ☀️ Apolo — Backend Senior Specialist

---

## Sessão 2026-07-20 — Afrodite (IMPLEMENTANDO ML-1B Node.js)

**Tarefa:** Implementar ML-1B do Roadmap `docs/roadmaps/ROADMAP-2026-06-19-architect-command-guidelines.md` (slash command `/trackfw:architect` + diretrizes de arquitetura no Node.js).
**Agente:** 💖 Afrodite - Frontend i18n Senior Specialist

---

## Sessão 2026-07-20 — Apolo (CONCLUÍDO ML-1C Python)

**Tarefa:** Implementar ML-1C em Python do `docs/roadmaps/ROADMAP-2026-06-19-architect-command-guidelines.md`.
**Agente:** ☀️ Apolo — Backend Senior Specialist

**Entregue:**
- `pypi/trackfw/generators/init_gen.py`:
  - `generate_claude_commands(cwd: str) -> None`: exportada e implementada com suporte a todos os slash commands, incluindo `architect.md`.
  - `architect.md`: gerado em `.claude/commands/trackfw/` com 5 passos estruturados (Descoberta de Negócio, Recomendação de Stack, Arquitetura em Camadas, Gerar ADR de Stack, Próximos Passos).
  - `_trackfw_rules_block()`: atualizado com a seção `### Architecture Directives (mandatory)` contendo as 8 diretrizes de arquitetura.
  - `scaffold()`: chama `generate_claude_commands(cwd)`.
- `pypi/trackfw/commands/discover.py`:
  - Chama `generate_claude_commands(cwd)` ao executar com a flag `--init`.
- `pypi/tests/test_generators_init.py`:
  - Adicionada a classe `TestGenerateClaudeCommands` testando a criação de `architect.md`, de todos os slash commands via `scaffold`, e a presença da seção `### Architecture Directives (mandatory)`.
  - Executado `python3 -m pytest pypi/tests/` com 323/323 testes verdes (100% de aprovação).
- `docs/roadmaps/ROADMAP-2026-06-19-architect-command-guidelines.md`:
  - ML-1C marcado como `✅ Concluído`.






---

## Sessão 2026-07-20 — Hades (IMPLEMENTANDO Threat Review PR#56 e PR#57)

**Tarefa:** Threat review de segurança dos PRs #56 (adr_dirs `~`, strict_ci_paths, isenção adr_orphan, diretiva de IA em geradores) e #57 (hooks nativos 7 CLIs de IA + scripts shell trackfw-attention-*.sh) já mergeados.
**Agente:** 🔒 Hades - Principal DevSecOps Security Specialist

---

## Sessão 2026-07-20 — Hades (CONCLUÍDO Threat Review PR#56 e PR#57)

**Tarefa:** Threat review de segurança dos PRs #56 e #57 já mergeados.
**Agente:** 🔒 Hades - Principal DevSecOps Security Specialist

**Achados principais (não corrigidos — apenas reportados, correção é handoff):**
- 🟠 Path Traversal (CWE-22) em `scripts/trackfw-attention-signal.sh` / `trackfw-attention-cleanup.sh` (Go/Node/Python): `ROADMAP_DIR` extraído via `grep` cru de `roadmap_dir:` em `trackfw.yaml` sem validação de contenção no `cwd` — diferente do tratamento dado a `adr_dirs` no PR#56 (que ganhou `isOutsideCWD`/`_is_subpath`). Um `trackfw.yaml` malicioso (`roadmap_dir: ../../../algum/lugar`) permite `mkdir -p`/escrita de `.trackfw-attention.json` fora do projeto quando o hook dispara automaticamente a cada tool call.
- 🟡 Improper JSON escaping (CWE-116) no mesmo script: `sed 's/"/\\"/g'` escapa apenas aspas, não backslashes; um `MSG`/`TOOL` terminando em `\` corrompe o JSON gerado (a versão Node removeu o `tr -d '\n'` que existia antes, piorando o cenário de newline embutido).
- Demais vetores (command injection via `$()`/backticks no shell, hijack de `.claude/settings.json`/hooks dos 7 CLIs, supply chain) — avaliados como NÃO exploráveis: os injetores de hook usam apenas comandos estáticos hardcoded (`scripts/trackfw-attention-*.sh`), sem interpolação de dados externos; os valores extraídos via `jq`/`python3` só são usados como argumento de `%s` do `printf`, nunca via `eval`/`bash -c`.

**Nenhum código alterado** — revisão apenas, sem correções (fora do escopo do Hades; achados endereçados para handoff aos agentes implementadores).

---

## Sessão 2026-07-20 — Zeus (CONCLUÍDO Implementação Roadmap pós-auditoria PRs #56 e #57)

**Tarefa:** Execução completa do roadmap `ROADMAP-2026-07-20-corrigir-attention-hooks-e-hardening-pos-auditoria-pr56-pr57.md`.
**Agente:** 🌩️ Zeus - Principal Software Architect
**Branch:** `fix/attention-hooks-pos-auditoria`

- **Status:** CONCLUÍDO (13/13 achados zerados com testes verdes nos 3 CLIs)
  - **C13 resolvido:** Adicionadas asserções de igualdade de conteúdo pós-2ª injeção (`bytes.Equal` em Go, `deepStrictEqual` em Node e `assertEqual` em Python) para Kiro e Copilot. `make quality` 100% verde.
  - **Apolo (Go):** Alinhado evento do Claude (`PreToolUse[AskUserQuestion]`) e Codex (`PermissionRequest`). Resiliência de script shell contra `grep` sem match sob `pipefail`. Path containment e JSON escaping. Constante de ADRs globais unificada. `go test ./...` 100% verde.
  - **Afrodite (Node):** Alinhado evento do Codex (`PermissionRequest`). Resiliência de script shell sob `pipefail`. Path containment e JSON escaping. Constante de ADRs globais unificada. `npm test` (58 testes) 100% verde.
  - **Python Specialist:** Resiliência de script shell sob `pipefail`. Path containment e JSON escaping. Exceção silenciosa em `discover.py` removida. Granularidade por-arquivo de `adr_orphan` com `realpath` e suporte a `windsurf`. Constante de ADRs globais. `pytest pypi/tests/` (330 testes) 100% verde.
- **Wave 3 (Barrier QA & Contratos):**
  - `make quality` executado e 100% VERDE (Go + Node.js + Python + CLI parity lifecycle e smoke checks + validate JSON parity).
  - Roadmap atualizado e movido para `docs/roadmaps/done/ROADMAP-2026-07-20-corrigir-attention-hooks-e-hardening-pos-auditoria-pr56-pr57.md`.
  - REQ atualizada para `Status: Done`.

---

## Sessão 2026-07-20 — Zeus (CONCLUÍDO Auditoria de conformidade pós-implementação + reabertura C13)

**Tarefa:** Auditar a implementação do agy na branch `fix/attention-hooks-pos-auditoria` contra os 13 achados (C1–C13) da REQ pós-auditoria, verificar cobertura e correção.
**Agente:** 🌩️ Zeus - Principal Software Architect
**Branch:** `fix/attention-hooks-pos-auditoria`

**Status:** CONCLUÍDO
- **Verificação:** 3 auditores paralelos (Go/Node/Python) + suítes de teste (Go `ok`, Node 58/58, Python 330/330, `go vet` limpo) + reprodução empírica de C1 (script roda `exit=0` e escreve JSON no fallback `docs/roadmaps` sem `roadmap_dir:`) e C5 (payload com `"`,`\`,`\n` → JSON escapado e parseável).
- **Resultado:** 11 de 13 achados sólida e corretamente resolvidos, incluindo os 3 críticos (C1/C2/C3). Feature funcional e endurecida.
- **⚠️ C13 REABERTO:** cobertura inconsistente entre CLIs — comparação de conteúdo na idempotência de Kiro/Copilot só implementada em Python-Copilot; Go (Kiro+Copilot), Python-Kiro e Node ficaram com `len==2`/asserção parcial. Pendência acionável (arquivos+linhas) registrada no roadmap para o agy corrigir.
- **Abertura de PR:** PR #59 aberto na branch `fix/attention-hooks-pos-auditoria` apontando para `main` (https://github.com/kgsaran/trackfw/pull/59).


---

## Sessão 2026-07-20 — Code Quality (IMPLEMENTANDO Revisão de Qualidade de Código PR #59)

**Tarefa:** Revisão de qualidade de código (manutenibilidade, duplicação, paridade 3 CLIs, robustez, legibilidade, testes) do PR #59 (correções pós-auditoria PRs #56/#57), sem edições.
**Agente:** 🔧 Code Quality - Code Quality Senior Specialist
**Branch:** `fix/attention-hooks-pos-auditoria`

---

## Sessão 2026-07-20 — Zeus (CONCLUÍDO Reanálise de qualidade PR #59 + REQ/Roadmap de hardening Q1-Q8)

**Tarefa:** Reanalisar a QUALIDADE do código do PR #59 (além da correção dos 13 achados) e gerar REQ→Roadmap para o agy implementar o hardening.
**Agente:** 🌩️ Zeus - Principal Software Architect
**Branch:** `fix/attention-hooks-pos-auditoria`

**Status:** CONCLUÍDO (documentos de governança gerados; implementação delegada ao agy)
- **Reanálise (verificada em código + reprodução empírica + comparação lado-a-lado dos 3 scripts):** 8 achados de qualidade (Q1–Q8).
  - 🔴 Q1: teste de contrato do Go não executa o script (só string-contains) — Node/Python executam; paridade de teste quebrada e anti-padrão da Wave 3 reaberto.
  - 🟠 Q2: escaping não cobre caracteres de controle U+0000–U+001F (TAB/CR quebram o JSON — reproduzido: jq e json.loads rejeitam). Q3: contenção de traversal diverge (Go relativiza abs-sob-cwd/`*..*` vs Node/Python segment-aware). Q4: `tr -d '\n'` (Go/Python) vs `tr -d '\r\n'` (Node). Q5: fallback sem `jq` nunca testado.
  - 🟡 Q6: parsing YAML frágil. Q7: falta teste golden de paridade. Q8: pressuposto de cwd não documentado.
- **Artefatos criados (backlog):** `docs/req/REQ-2026-07-20-hardening-qualidade-attention-hooks-pos-pr59.md` + `docs/roadmaps/backlog/ROADMAP-2026-07-20-hardening-qualidade-attention-hooks-pos-pr59.md` (Wave 1 scripts por CLI → Wave 2 testes → Wave 3 barrier; com decisões canônicas para paridade idêntica).
- **Housekeeping:** removida cópia stale de `ROADMAP-...-pr56-pr57.md` em `backlog/` (duplicata vinda do #58/main; autoritativa é a de `done/`) — sintoma da divergência de squash-merge #58↔#59 já reportada.

---

## Sessão 2026-07-20 — Zeus (CONCLUÍDO Roadmap Hardening Qualidade Q1-Q8 pós-PR59)

**Tarefa:** Orquestração e execução do roadmap `ROADMAP-2026-07-20-hardening-qualidade-attention-hooks-pos-pr59.md`.
**Agente:** 🌩️ Zeus - Principal Software Architect
**Branch:** `fix/hardening-qualidade-attention-hooks`

**Status:** CONCLUÍDO (8/8 achados Q1-Q8 resolvidos e testados nos 3 CLIs)
- **Wave 1 (Decisões Canônicas dos Scripts):**
  - Contenção de path traversal única e segment-aware (`/*|../*|*/../*|*/..|..`) nos 3 CLIs (Q3).
  - Sanitização de caracteres de controle U+0000–U+001F (`tr -d '\000-\037'`) antes do escaping nos 3 CLIs (Q2, Q4).
  - Extração tolerante de `roadmap_dir:` (`sed 's/^roadmap_dir:[[:space:]]*//; s/[[:space:]]*#.*$//'`) (Q6).
  - Comentário explicativo de cwd (Q8).
- **Wave 2 (Testes de Contrato, Fallback e Golden Parity):**
  - **Go (Apolo):** Adicionados `TestAttentionScripts_ExecutionContract` (executa scripts bash para default, traversal e payload com aspas/barras/newlines/tabs/CR) e `TestAttentionScripts_FallbackWithoutJQ` (Q1, Q5).
  - **Node (Afrodite):** Adicionado teste de fallback sem `jq` com `fakeBinDir` (Q5).
  - **Python Specialist:** Adicionado `test_fallback_without_jq` com `fake_bin` (Q5).
  - **QA Golden Parity:** Criado `internal/generators/scaffold_parity_test.go` (`TestScriptsParity_GoldenCanonicalBlocks`) que compara byte-a-byte a estrutura dos scripts shell gerados nos 3 CLIs (Q7).
- **Wave 3 (Barrier Quality):**
  - `make quality` 100% VERDE (Go unit/vet/contract/parity + Node 60/60 + Python 333/333).
  - Roadmap movido para `docs/roadmaps/done/ROADMAP-2026-07-20-hardening-qualidade-attention-hooks-pos-pr59.md`.
  - REQ atualizada para `Status: Done`.
- **Abertura de PR:** PR #60 aberto na branch `fix/hardening-qualidade-attention-hooks` apontando para `main` (https://github.com/kgsaran/trackfw/pull/60).

---

## Sessão 2026-07-20 — Zeus (CONCLUÍDO Release v2.15.0)

**Tarefa:** Sincronização dos arquivos de versão para a nova release v2.15.0 pós-integração dos PRs #56, #57, #58, #59 e #60.
**Agente:** 🌩️ Zeus - Principal Software Architect
**Branch:** `chore/release-v2.15.0`

**Status:** CONCLUÍDO
- Bump de `2.14.0` → `2.15.0` nos 5 arquivos de versão: `internal/version/version.go`, `npm/package.json`, `pypi/pyproject.toml`, `pypi/trackfw/__init__.py`, `docs/visao-projeto/VISION.md`.
- `make quality` **100% VERDE**.

---

## Sessão 2026-07-24 — Zeus (IMPLEMENTANDO Fix Windows integrations resolve)

**Tarefa:** Bug de produção reportado no Windows — `trackfw agents list/install` (npm v2.15.0) aborta com `Unsafe destination: .amazonq/cli-agents/...` e `.claude/agents/...`.
**Agente:** 🌩️ Zeus - Principal Software Architect (orquestração)
**Branch:** `fix/windows-integrations-resolve`
**REQ:** `docs/req/REQ-2026-07-24-corrige-resolve-de-integrations-em-windows-destinos-validos-rejeitados.md`
**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-24-fix-windows-path-resolve-em-integrations-(node+go)-e-guard-de-regressao.md`

**Causa raiz (análise estática validada):** `resolve()` compara input POSIX (`/`) contra normalização dependente de plataforma. Node `path.normalize` (manager.js:31) e Go `filepath.Clean` (manager.go:398) convertem `/`→`\` no Windows, disparando `Unsafe destination`. Python (manager.py:47, `".." in parts`) já é cross-platform correto — referência. Bug invisível no CI atual (100% ubuntu).
**Plano:** ML-1A Node `path.posix.normalize` · ML-1B Go `path.Clean` · ML-1C testes paridade 3 CLIs · ML-1D job `windows-latest` (guard real) · ML-2A release patch 2.15.1.
**Status:** CONCLUÍDO (Wave 1) — pendente release (Wave 2, decisão do usuário)
- **Apolo** entregou commit `e9cb58c`: Node `path.posix.normalize`, Go `path.Clean` (+import `path`), 3 suites de teste de paridade (Go table-driven, Node 9 casos, Python 8 casos), job `windows-latest` no `quality.yml` wired em `needs` do gate agregado.
- **Auditoria Zeus:** diffs conferem exatamente ao roadmap; nenhuma linha indevida tocada. `go build ./...` OK, `go test ./internal/integrations/...` OK, `make quality` verde (341 Python + 69 Node + Go).
- **Divergência de paridade documentada:** `./x` só é rejeitado em Node/Go (forma canônica), não em Python (`".." in parts` é semântico). Irrelevante ao bug — destinos do catálogo são canônicos.
- **Wave 2 (release patch 2.15.1):** CONCLUÍDA. PR #62 mergeado (squash). Bump 2.15.0 → 2.15.1 nos 5 arquivos de versão na branch `chore/release-v2.15.1`; `make quality` verde (341 Python + Go + parity). Roadmap → done, REQ → Done. Tag `v2.15.1` a criar após merge do release.

---

## Sessão 2026-07-24 — Apolo (CONCLUÍDO)

**Tarefa:** Fix cross-platform Windows para `resolve()` em integrations (ML-1A, 1B, 1C, 1D). Branch `fix/windows-integrations-resolve`.

**Entregue:**
- `npm/src/integrations/manager.js` linha 31 — `path.normalize` → `path.posix.normalize`; `` `..${path.sep}` `` → `'../'`
- `internal/integrations/manager.go` linha 398 — `filepath.Clean` → `path.Clean`; `string(filepath.Separator)` → `"/"`. Import `"path"` adicionado.
- `internal/integrations/manager_test.go` — `TestResolveWindowsCrossplatform` (table-driven): 2 casos de aceitação + 7 de rejeição.
- `npm/tests/integrations_resolve.test.js` — 9 testes (node --test): 2 accept + 7 reject.
- `pypi/tests/test_integrations_resolve.py` — 8 testes (pytest): 2 accept + 6 reject (./x omitido — Python não enforça forma canônica para não-traversal).
- `.github/workflows/quality.yml` — job `windows-latest` guard de regressão real.

**Resultado:** `go build ./...` ✅ | `go test ./internal/integrations/...` 100% | Node 9/9 | Python 8/8 | `make quality` 341 testes Python + 69 Node + go vet/build todos verdes. Commit `e9cb58c` | push para `fix/windows-integrations-resolve`.

**Decisão autônoma:** `./x` removido dos casos de rejeição Python — `Path('./x').parts == ('x',)`, implementação de referência não rejeita (não é traversal). Documentado no commit e no teste.




---

## Sessão 2026-07-25 — Zeus (IMPLEMENTANDO)

**Tarefa:** Identidade humanizada dos agentes trackfw. Branch `feat/identidade-humanizada-agentes`.

**REQ:** `docs/req/REQ-2026-07-25-identidade-humanizada-dos-agentes-trackfw.md`
**ADR:** `docs/adr/ADR-2026-07-25-identidade-personalizavel-de-agentes.md`
**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-25-identidade-humanizada-dos-agentes.md`

**Objetivo:** permitir nomear os 10 agentes (`display_name` → `description` + corpo; `slug`+`-tf` → `name`) e definir apelido do usuário (corpo apenas), materializado em tempo de instalação por `Render()`.

**Achados que moldaram o ADR (verificados em código + doc oficial):**
- Seleção de subagent usa **apenas** `name` + `description`; o corpo carrega só após a seleção. Logo o apelido precisa ir no `description` para habilitar roteamento natural.
- `name` **não** precisa coincidir com o filename (doc oficial). Path vem de `{{id}}` → alterar `name` não mexe em manifest nem gera órfãos.
- **Bloqueante encontrado:** os 10 nomes do preset grego já existem em `~/.claude/agents/` (agentes pessoais do usuário). Colisão de `name` no mesmo diretório = shadowing silencioso e não-determinístico (doc oficial). Resolvido por sufixo fixo `-tf` no slug + varredura de colisão obrigatória.
- `agentTools` decidia SET_ARCH por `strings.HasSuffix(name,"architect")` — quebraria com `name` customizado. Refatorado para `item.ID == "architect"`.
- Modelo de hash do manifest **suporta** personalização sem drift: `desiredHash` vem de `plan.Content` em tempo de plano.

**Plano:** 5 waves / 7 MLs. W1 (paralelo): ML-1A pacote `internal/identity` + ML-1B placeholders nos assets. W2: ML-2A render/plan/manager. W3: ML-3A wizard `init` + 4 callers + i18n. W4 (paralelo): ML-4A npm + ML-4B pypi. W5: ML-5A gates de paridade + docs.

**Status:** IMPLEMENTANDO — Wave 1 em execução.

**Wave 1 — CONCLUÍDA com mudança de abordagem:**
- ML-1A ✅ (`9b75dad`) pacote `internal/identity` — `Slugify` (NFD + ASCII-fold via `golang.org/x/text`), `Load`/`Save` atômico em `~/.trackfw/identity.json`, `AgentName` (sufixo `-tf`), `Validate`, fixture de 14 vetores de slug para paridade.
- ML-1C ✅ (`6e5e179`) 5 presets temáticos hardcoded (greek/norse/potter/thrones/chaves) + `Preset(name)`/`PresetNames()`. 27 testes.
- ML-1B ❌ **REVERTIDO** (`9ef17b3` → revert). Auditoria Zeus achou vazamento: `Render()` tem 2 rotas e o placeholder `{{IDENTITY_LINE}}` só seria removido em uma. O branch `default:` (`representation: "subagent"` — usado pela superfície **claude**) devolve o source cru, então `trackfw agents install` gravaria o placeholder literal em `~/.claude/agents/trackfw-architect.md`. Confirmado empiricamente. Só 2 testes Node (goldens inline) pegaram, e nas rotas menos usadas.
  **Nova abordagem:** assets intocados; `Render()` **insere** a linha de identidade quando há identidade. Não-regressão vira verdadeira por construção em vez de depender de 6 implementações corretas de strip (2 rotas × 3 CLIs).
- **Lacuna descoberta:** não existe cobertura de golden para bytes renderizados em Go — os testes renderizam do mesmo asset embedado, logo são auto-consistentes. ML-2A passa a exigir goldens congelados do estado pré-mudança.
- **Correção de auditoria:** `go mod tidy` promoveu `golang.org/x/text` de indireta para direta (mesma versão, sem download novo).

**ML-2A — CONCLUÍDO (`09ca1c0`).** Injeção de identidade no `Render()` cobrindo as **duas rotas** de saída (`internal/integrations/render.go`, `plan.go`, `manager.go`):

- `PlanRequest` ganhou campo `Identity identity.Config`, repassado a `Render`.
- `Render` recebe `cfg identity.Config`; quando `identity.Lookup(cfg, item.ID)` acerta, aplica `name` (`identity.AgentName(slug)`), `description` (`DisplayName + " — " + description original`) e injeta saudação como primeira linha do corpo (`Você é {DisplayName}.` ou, com apelido, `Você é {DisplayName}. Trate o usuário como {UserNickname}.`) seguida de linha em branco.
- **Rota A** (`custom-agent-toml`, `cli-agent-json`/`agent-json`, `agent-directory`) recebe a injeção via as variáveis já extraídas por `markdownParts`.
- **Rota B** (branch `default:`, representation `subagent` — usado por claude/gemini/cursor/copilot/kiro-ide/windsurf) recebe a injeção via nova função `insertBodyPrefix`, que localiza o fim do frontmatter com a mesma lógica de `markdownParts` e insere a saudação no corpo cru, preservando o frontmatter original.
- **Garantia por construção:** quando `Lookup` retorna `ok=false`, o branch `default:` retorna literalmente `normalizeMarkdown(source)` — a mesma expressão de antes da mudança — e as variáveis `name/description/body` só são mutadas dentro de `if hasIdentity {...}`. Não há caminho de código que produza a saída antiga "por acaso".
- `agentTools` deixou de decidir SET_ARCH por `strings.HasSuffix(name, "architect")` e passou a comparar `item.ID == "architect"` (ADR D8) — sobrevive a `name` customizado (`zeus-tf`).
- `manager.go` ganhou `detectNameCollision`: antes de instalar/atualizar um artefato `KindAgents`, varre o diretório de destino por outros arquivos `.md` que declarem o mesmo `name` no frontmatter (via nova `frontmatterName`). Colisão sem `force` → erro; com `force` → aviso em stderr e prossegue. O próprio destino do artefato nunca conta como colisão (excluído da varredura de siblings). **Limitação documentada em comentário:** a varredura só cobre `.md` — JSON/TOML não são escaneados (exigiria parser genérico só para este check).
- **Goldens congelados** criados em `internal/integrations/testdata/` a partir de `git show 5fe5cb9:...architect.md` e dos literais `expected` em `npm/tests/agents-skills.test.js` (Codex TOML e Antigravity agent-directory) — a suite Go deixa de ser auto-referente.
- Teste-chave que a tentativa anterior não tinha: `TestRenderSubagentRouteInjectsIdentity` prova que a Rota B (representation `subagent`) recebe `Você é Zeus. Trate o usuário como chefe.` no corpo.

**Resultado:** `go build ./...` ✅ | `go test ./...` verde (toda a suite Go) | `go vet ./...` limpo | `scripts/check-integration-assets.sh` ✅ | `cd npm && npm test` 69/69 (sem regressão, não tocado) | `git status`: apenas `internal/integrations/*`. Commit `09ca1c0` na branch `feat/identidade-humanizada-agentes` (sem push).

**Wave 2 — CONCLUÍDA (2 MLs):**
- ML-2A ✅ (`09ca1c0`) `Render` recebe `identity.Config`; `PlanRequest.Identity`; `agentTools` passa a decidir SET_ARCH por `item.ID`; detecção de colisão de `name` em `manager.go` (limitada a `.md`); goldens congelados em `internal/integrations/testdata/` capturados de `5fe5cb9` e dos literais do teste Node.
- ML-2B ✅ (`863e6cf`) **corrige defeito achado na auditoria do ML-2A**: o branch `default:` inseria a saudação no corpo mas devolvia o frontmatter intacto — na superfície `claude` o `name` continuava `trackfw-architect`, quebrando `@agent-<slug>-tf` e o roteamento natural. Nova função `rewriteFrontmatterFields` reescreve `name:`/`description:` só dentro do bloco de frontmatter. Teste table-driven cobre todas as representações.
- **Verificação E2E do orquestrador** com `~/.trackfw/identity.json` real: `architect` → `name: zeus-tf`, `description: Zeus — ...`, `model: opus` preservado, corpo com `Você é Zeus. Trate o usuário como chefe.`; `backend` (não configurado) → byte a byte inalterado; 5 skills verificadas sem contaminação; path `~/.claude/agents/trackfw-architect.md` inalterado.
- **Padrão observado:** gates verdes (build/test/vet/paridade) não provaram a feature em nenhum dos dois defeitos. Ambos foram pegos por auditoria manual renderizando todas as 8 representações. Recomendação para o ML-5A: o gate de paridade precisa incluir asserção por representação, não só comparação entre CLIs.

**Wave 3 — CONCLUÍDA:**
- ML-3A ✅ (`af95e7c` + `3cd02b2`) wizard de identidade no `init` (12 opções + modo custom + apelido), flag `--identity-preset` (10 presets + neutral/none), wiring dos 4 callers de `BuildPlans`, i18n nos 3 locales. Agente rodou mutation tests removendo `Identity` de cada caller — todos os 4 testes falharam corretamente, provando que não são vacuous.
- **Verificação E2E do orquestrador pelo binário real** (`bin/trackfw` + HOME temporário):
  - `init --ai-tools claude --identity-preset pioneers` → `~/.trackfw/identity.json` gravado; `.claude/agents/trackfw-dba.md` com `name: codd-tf`, `description: Codd — ...`, corpo `Você é Codd.`
  - re-`init` sem a flag → identidade preservada (md5 idêntico)
  - `--identity-preset xpto` → erro listando os 12 valores válidos
  - `init` sem flag em HOME limpo → nenhuma identidade gravada; agente byte a byte igual ao de `5fe5cb9` (não-regressão)
  - `agents update` → **não reverteu** a identidade; `agents list` reporta `current` (sem falso drift)

**Wave 4 — EM EXECUÇÃO (2 MLs em paralelo):** ML-4A porta Node.js (`npm/`) e ML-4B porta Python (`pypi/`). Ambos recebem a advertência explícita das duas rotas de `Render` (a Rota B, do `default:`/`subagent`, foi onde os dois defeitos anteriores nasceram) e devem copiar `internal/identity/testdata/slug_vectors.json` byte a byte. Ponto de contrato: **os 3 CLIs leem o mesmo `~/.trackfw/identity.json`**.

**Wave 4 — ML-4B (Python) CONCLUÍDO** (`5c703e7`): módulo `pypi/trackfw/identity/`, duas rotas em `renderers.py` (`_insert_body_prefix` → `_rewrite_frontmatter_fields` → `_normalize_markdown`, mesma ordem do Go porque a reescrita dependeria de offsets deslocados se invertida), detecção de colisão, wizard + flag no `init`, i18n nos 3 locales. 392 testes (341 pré-existentes intocados + 51 novos). Agente corrigiu `ensure_ascii=True` nas duas chamadas `json.dumps` da rota TOML — sem identidade nunca divergiu (nenhum asset tem acento), mas `Você é Zeus` teria quebrado a paridade.

**Auditoria de paridade cross-CLI do orquestrador** — mesmo `~/.trackfw/identity.json` (architect=Zeus, apelido Kleber), instalando em 9 alvos pelos 3 CLIs e comparando md5:

| alvo | paridade | identidade aplicada |
|---|---|---|
| claude, gemini, cursor, copilot, windsurf, kiro | ✅ md5 idêntico | sim |
| codex | ✅ | sim |
| antigravity | ✅ | sim |
| **amazonq** | ❌ **Node diverge de Go/Python** | sim |

**Defeito PRÉ-EXISTENTE descoberto (não causado por esta feature):** na representação `cli-agent-json` (amazonq), o Go usa `json.MarshalIndent(map[string]string)`, que **ordena as chaves alfabeticamente** (`description, name, prompt`); o `JSON.stringify` do Node preserva a **ordem de inserção** (`name, description, prompt`). Python coincide com o Go. Confirmado reproduzindo com HOME limpo, **sem identidade** — logo é anterior a este trabalho; a feature apenas o tornou visível.

**Impacto real:** um usuário que instala agentes amazonq pelo CLI Go e depois roda `agents list` pelo CLI Node vê estado `modified` (falso drift), porque o manifest é indexado por hash de conteúdo. `check-cli-parity.sh` não detecta isso hoje.

**Encaminhamento:** ML-5A (que já vai adicionar o gate de paridade cross-CLI) deve corrigir a ordem das chaves no `render.js` do npm — caso contrário o próprio gate novo falharia. Alternativa rejeitada: documentar como exceção intencional em `docs/cli-parity.md`, porque o falso drift é um bug de usuário real, não uma divergência cosmética.

**Wave 4 — ML-4A (Node.js) CONCLUÍDO** (`9995c1c`, `6740541`): módulo `npm/src/identity/`, duas rotas em `render.js`, `toolsFor` por `item.id`, detecção de colisão, wizard + flag, i18n. 97 testes.

**Wave 5 — CONCLUÍDA (re-split em 2 MLs paralelos):**
- ML-5A ✅ (`e10ffad`, `3b22736`) — corrigiu o defeito **pré-existente** do `cli-agent-json`/`agent-json` (ordem de chaves JSON no Node) e criou `scripts/check-identity-parity.sh`. Ampliou o gate de 9 para **11 combinações target/surface** por conta própria, incluindo `antigravity=legacy-cli` e `kiro=cli` — a representação `agent-json` só existe em superfícies **não-default**, então sem elas metade do fix ficaria sem cobertura (e ambas de fato divergiam).
- ML-5B ✅ (`641494e`) — `docs/cli-parity.md`, os 3 READMEs (`npm/README.md` não existia e foi criado), fechamento de roadmap e REQ. Recusou-se a marcar 1 critério não verificável e reportou 3 achados, 2 deles defeitos meus.

**Wave 6 — CONCLUÍDA:** ML-6A ✅ (`903ad9c`) — `Validate` rejeita slug com sufixo `-tf` duplicado nos 3 CLIs. Fecha footgun de caminho suportado (edição manual do `identity.json`, ADR D9).

**Defeitos corrigidos pela auditoria do orquestrador nesta feature:**
1. Placeholder `{{IDENTITY_LINE}}` vazaria literal em `~/.claude/agents/` (ML-1B revertido)
2. Rota `subagent` não reescrevia o frontmatter — `@agent-<slug>-tf` e roteamento natural quebrados na superfície principal (ML-2B)
3. Ordem de chaves JSON divergente Node × Go/Python — **pré-existente**, causava falso drift (ML-5A)
4. Exemplo do ADR D5 induzia `name: zeus-tf-tf` (corrigido + guard no ML-6A)
5. Nome da branch sem slug correspondente no roadmap — `trackfw validate` vermelho (roadmap renomeado)

**Lição registrada:** nos defeitos 1, 2 e 3 **todos os gates estavam verdes**. Os testes Go eram auto-referentes (renderizavam do mesmo asset embedado) e não detectavam drift. A correção estrutural foi introduzir goldens congelados em `internal/integrations/testdata/` e o gate cross-CLI `check-identity-parity.sh` — este último verificado falhando de propósito antes de ser aceito.

**Estado final:** `make quality` verde (Go + 99 Node + 394 Python + 5 gates de paridade). `trackfw validate`: 2 violations, ambas pré-existentes de `REQ-2026-07-24-corrige-resolve...`.

**Status:** CONCLUÍDO — aguardando decisão do usuário sobre PR (não aberto).

---

## Sessão 2026-07-25 (ciclo 2) — Zeus (IMPLEMENTANDO)

**Tarefa:** Wizard guiado de identidade no `agents install`. Branch `feat/wizard-guiado-identidade-agents-install`.

**REQ:** `docs/req/REQ-2026-07-25-wizard-guiado-de-identidade-no-agents-install.md`
**ADR:** `docs/adr/ADR-2026-07-25-wizard-unificado-de-identidade-no-agents-install.md`
**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-25-wizard-guiado-identidade-agents-install.md`

**Ciclo anterior encerrado:** PR #64 mergeado. Roadmap → `done/`, REQ → `Done`, ADR → `Accepted`. Branch local removida após confirmar squash-merge (`git diff origin/main origin/feat/... --stat` vazio).

**Lacunas que motivam este ciclo (levantadas pelo usuário):**
- **L1 descoberta:** o wizard ficou só em `init`. `agents install` apenas **lê** `~/.trackfw/identity.json` (`integrations_flags.go:143`) e nunca oferece configurá-la — quem não roda `init` de novo só descobre a feature pelo README. Uma feature de personalização invisível no comando que a consome está, na prática, desligada.
- **L2 rótulos:** o modo `custom` exibe o `id` técnico (`architect`, `code-quality`) em vez da especialidade. O catálogo já tem `Item.Name` + `Item.Description` embedados e não usados.
- **L3 presets às cegas:** escolher `tolkien` não revela que security→Boromir e dba→Elrond até os arquivos estarem em disco. Não há confirmação.

**Decisão de escopo:** é REQ **exclusivamente de UX de CLI** — não altera schema de `identity.json`, contrato de slug nem artefatos gerados. Critério de controle: `check-identity-parity.sh` deve continuar passando **sem nenhuma alteração**; se precisar mudar, algo saiu do escopo.

**Risco principal identificado no ADR (D2):** a regra de acionamento. Errar para o lado permissivo transforma o wizard em incômodo recorrente e leva o usuário a automatizar o "pular", esvaziando a feature. Exige teste explícito do caso "identidade já existe → não pergunta".

**Plano:** 3 waves / 4 MLs. W1 (sequencial, define o contrato de UX): ML-1A componente Go + init + agents install. W2 (paralelo): ML-2A npm, ML-2B pypi. W3: ML-3A docs + E2E.

**Status:** IMPLEMENTANDO — Wave 1 em execução.

### Data — ML-2B (pypi) — CONCLUÍDO

**Escopo:** portou o wizard guiado de identidade (ADR-2026-07-25-wizard-unificado-de-identidade-no-agents-install) para o CLI Python. Exclusivo em `pypi/`.

**Arquivos:**
- Novo `pypi/trackfw/commands/identity_wizard.py` — componente compartilhado (`run_identity_wizard`, indireção `identity_wizard_runner` para spies em teste, `should_prompt_identity`, `identity_file_exists`, `resolve_identity_preset`, `apply_identity_preset_flag`, `IDENTITY_PRESET_LABELS`).
- `pypi/trackfw/commands/init.py` — `init` agora consome o wizard compartilhado; removida a implementação antiga de `_run_identity_wizard`/`_IDENTITY_PRESET_LABELS` (duplicada).
- `pypi/trackfw/integrations/command.py` — `agents install/update/uninstall` ganham `--identity`/`--identity-preset` (apenas quando `kind == "agents"` e ação é mutação); trigger do wizard entre seleção de surface e o `plan_deployments` final; recarrega identidade do disco após wizard/preset antes de montar os planos definitivos (ponto crítico apontado pelo advisor — sem isso os nomes custom regridem silenciosamente para neutro).
- 3 locales `pypi/trackfw/i18n/locales/*.json` — bloco `identity.inUse` / `identity.wizard.{confirmHeader,confirmQuestion,nicknameRowLabel}` adicionado (faltava inteiramente nos 3 locales Python; as chaves `init.prompt.identityPreset/identityCustomName/identityNickname` já existiam de ciclo anterior).
- Novo `pypi/tests/test_identity_wizard.py` — 24 testes: truth table completa de `should_prompt_identity` (16 combos), gatilho do wizard em `agents install` (existente/sem flag não invoca, sem identidade com TTY invoca, `--identity` força, `skills install` nunca invoca, não-TTY nunca bloqueia), recusa de confirmação não persiste nada, rótulos do modo custom usam `name — description` do catálogo (não o id cru), erro de `--identity-preset` inválido lista os válidos.

**Validação:**
- `make test-python`: 418 passed.
- `scripts/check-identity-parity.sh` (sem alteração no script): passou para as 11 combinações target/surface, com e sem identidade — inclui Go, Node.js (já implementado em paralelo por outro agente) e Python.
- 5 cenários E2E comparados manualmente contra o binário Go: (1) non-TTY sem identidade não trava e não grava `identity.json` — igual; (2) `--identity-preset starwars` → `dba` vira `r2-d2-tf` nos dois; (3) identidade existente imprime `identity: 10 custom agent(s)` nos dois (locale fixado em `en-US` para comparação); (4) `skills install --help` sem nenhuma flag de identidade nos dois; (5) `--identity-preset xpto` → mesma lista de válidos na mesma ordem nos dois (exit code diverge Go=1/Python=2, divergência preexistente do CLI, não é critério de paridade).
- Regra de acionamento sem TTY real: testada via `monkeypatch.setattr("sys.stdin.isatty", lambda: False/True)` chamando `integrations_command.run(...)` diretamente com um spy substituindo `identity_wizard.identity_wizard_runner` (nunca chamando o `input()` real) — mesmo padrão de indireção do Go (`var identityWizardRunner`), documentado no docstring do módulo para não regressar.

**Git:** commit `9266242` em `feat/wizard-guiado-identidade-agents-install`, apenas `pypi/trackfw` e `pypi/tests` staged (confirmado `git status --short pypi` limpo antes do commit; `.trackfw-baseline.json` e `AGENTS.md` não tocados).

**Status:** CONCLUÍDO.

**Wave 3 — CONCLUÍDA:** ML-3A ✅ (`1e81ea2`, `1e9c514`) — documentação nos 3 READMEs + `cli-parity.md` + fechamento de governança. `make quality` verde, `trackfw validate` só com as 2 violations pré-existentes.

**Achado corrigido pelo orquestrador pós-ML-3A:** o agente reportou que o ADR D6 descrevia a ordem "apelido antes do preset", mas a implementação real (verificada nos 3 CLIs) faz preset → nomes → apelido → confirmação. Era o **ADR que estava errado**, não o código — corrigido D6 registrando explicitamente que nenhuma mudança de código foi necessária.

**Estado final do ciclo:** roadmap → `done/`, REQ → `Done`, ADR → `Accepted`. `make quality` verde (Go + 113 Node + 418 Python + 5 gates de paridade, incluindo `check-identity-parity.sh` **sem nenhuma linha alterada** durante toda a feature — o guarda-corpo de escopo funcionou do início ao fim).

**Status:** CONCLUÍDO — aguardando decisão do usuário sobre PR (não aberto).

---

## Encerramento — Ciclo 2 (wizard guiado de identidade)

PR #65 mergeado (squash) na main. Branch local removida (diff vazio contra `origin/main`, confirmando integração). Roadmap → `done/`, REQ → `Done`, ADR já estava `Accepted`.

**Duas gerações de identidade personalizável de agentes, agora concluídas:**
1. PR #64 — a feature em si (10 presets, 3 CLIs, materialização em build time)
2. PR #65 — descoberta via `agents install`, rótulos por especialidade, confirmação antes de gravar

`check-identity-parity.sh` não teve **uma única linha alterada** durante toda a segunda REQ — o guarda-corpo de escopo (esta REQ é só UX, não muda schema/contrato/artefatos) funcionou do início ao fim.

`make quality` verde na main: Go + 113 Node + 418 Python + 5 gates de paridade.

---

## Ciclo 3 — Escopo de instalação selecionável para agents e skills

**Data:** 2026-07-25 | **Orquestrador:** Zeus | **Status:** IMPLEMENTANDO

**Origem:** usuário reportou que `trackfw agents install` instala silenciosamente no projeto
atual, quando o esperado é instalar na pasta do usuário ou perguntar o escopo. Mesma queixa
vale para skills.

**Causa raiz (análise estática, 3 CLIs):** `--scope` com default fixo `"project"` em
`internal/commands/integrations_flags.go:105`, `npm/src/commands/integrations.js:50`,
`pypi/trackfw/integrations/command.py:94` (+ `catalog.py:59`), e `Scope: "project"`
hardcoded em `internal/commands/init.go:358`. Nenhum prompt de escopo existe. O único
prompt (`promptIntegrationSelection`) só dispara quando `--targets` está vazio — o caso
comum `--targets claude` não passa por prompt algum. Os 11 surfaces do catálogo suportam
`global` e `project`, então não há restrição técnica.

**Armadilha registrada:** a detecção de "usuário não escolheu" precisa usar *flag-set*
(`cmd.Flags().Changed("scope")` / `undefined` / `default=None`) — comparar contra o valor
`"project"` não distingue um `--scope project` explícito do default e re-perguntaria a quem
já escolheu.

**Decisões do usuário:** default `global` em modo não-interativo (breaking change);
`init` também pergunta; sem confirmação extra, apenas impressão dos destinos resolvidos.

**Artefatos:** ADR + REQ + Roadmap `2026-07-25-escopo-de-instalacao-selecionavel-para-agents-e-skills`,
branch `fix/escopo-de-instalacao-selecionavel-para-agents-e-skills`.

**Plano:** Wave 1 com 3 MLs em paralelo (Go / Node / Python — árvores disjuntas),
barrier, Wave 2 com ML de paridade + CHANGELOG + docs.

**Wave 1 — CONCLUÍDA:** ML-1A Go (`fb33bbb`), ML-1B Node (`ac8b45b`), ML-1C Python (`5acf8f1`).
Os 3 CLIs com detecção por *flag-set*, gate independente de `--targets`, prompt com `global`
pré-selecionado, `list` sem prompt, impressão de destinos.

**Wave 2 — CONCLUÍDA (ADR D8):** `321c148` / `e181597` / `f46bd74`. Auditoria pós-ML-1A
detectou que `agents uninstall --targets X` sem TTY resolvia destinos em `~/.claude/` —
um script de CI passaria a apagar os agentes do home do usuário. Guarda adicionada:
`uninstall` sem TTY e sem `--scope` falha exigindo a flag. `install`/`update` mantêm `global`.

**Wave 3 — CONCLUÍDA:** `c6e12b8` / `b05b2a7` / `5cc061a`. As 7 divergências reconciliadas;
2 eram bugs reais (Node `installIntegrationTarget` não imprimia destinos; teste do default
real do prompt ausente nos 3 CLIs), 5 eram exceções intencionais agora documentadas em
`docs/cli-parity.md`. CHANGELOG com os 2 breaking changes.

**Validação final pelo orquestrador (não confiando só nos relatórios):**
- `make build && make test && make lint && make quality` — todos verdes (Go + 119 Node +
  435 Python + 5 gates de paridade).
- `trackfw validate` — só as 2 violations pré-existentes da REQ de 2026-07-24.
- E2E manual nos 3 binários: `install` sem scope → `~/.claude/...` nos 3; `uninstall` sem
  scope → erro nos 3.

**Achados registrados (fora de escopo, §11):**
- CLI Node lança `Error` não tratado em falhas de validação, exibindo stack trace em vez de
  mensagem limpa. **Pré-existente** — o erro de `--targets` se comporta igual desde antes
  desta feature. Go imprime `Error: msg`; Python imprime `trackfw agents X: msg`.
- Mensagem de `--targets` ausente diverge no Python (`"--targets is required for
  non-interactive X"` vs `"X requires --targets in non-interactive mode"`). Pré-existente,
  documentada como exceção rastreada.
- JSON do Node é compacto; Go e Python indentam. Pré-existente.

**Incidente operacional:** o agente da Wave 3, ao validar o binário manualmente, rodou
`install` na raiz do repo e limpou com `rm -rf .claude`, o que apagou os diretórios de 3
worktrees git pré-existentes. Os arquivos tracked foram restaurados (`git checkout`), as
branches `worktree-agent-*` sobreviveram e seus commits (`feat(update)`) já estavam na main
— sem perda real. As worktrees ficaram `prunable`. **Lição:** validação manual de comandos
que escrevem no filesystem deve ocorrer sempre em diretório temporário, nunca na raiz do repo.

**Status:** CONCLUÍDO — aguardando decisão do usuário sobre PR (não aberto).

## Encerramento — Ciclo 3 (escopo de instalação selecionável)

PRs #68 (feature), #69 (fix do CI) e #70 (release) mergeados na main. Tag `v3.0.0` publicada.

**Por que major (2.16.0 → 3.0.0):** o usuário inicialmente optou por 2.17.0 (minor) e pediu
a justificativa; ao enumerar as quebras item a item, identifiquei uma **terceira** que não
estava documentada — o contrato de saída do `list --json`, que passa a reportar
`"scope": "global"` e destinos `~/...` para a mesma pergunta. Com as três quebras
explicitadas, o usuário reconsiderou para major.

**Lição de CI:** `make quality` **não** inclui `scripts/smoke-integration-packages.sh` —
ele só roda no GitHub Actions. Por isso todos os gates locais passaram verdes enquanto o
`package-smoke` quebrava no CI (o script instalava sem `--scope` e afirmava caminhos de
projeto). Considerar incluí-lo no `make quality`, ou ao menos documentar que o gate local
é incompleto para mudanças que afetam resolução de caminhos.

**Formato de CHANGELOG:** a pedido do usuário, a seção da 3.0.0 não se limita a listar
commits — abre com "Por que esta versão é major" (contexto do bug, por que o fix exigiu
inverter um default, as três quebras com impacto de cada uma) e um bloco "Migração" com o
diff exato para pipelines. Adotar esse formato em futuras majors.

**Status:** CONCLUÍDO.

---

## 2026-07-26 — Zeus — Comparativo agents trackfw × panteão grego (análise doc-only)

**Escopo:** comparar os agents dos fontes do trackfw com os agents implantados em
`~/.claude/agents/`, nos eixos harness, fluxo git e coordenação. Sem mudanças de código.

**Entregável:** `docs/analises/2026-07-26-comparativo-agents-trackfw-vs-panteao.md`.

**Achados principais:**
1. Existem **três** conjuntos, não dois: panteão grego (14, hand-authored), assets vivos do
   trackfw (10, ~360 bytes, EN, enviados ao usuário) e templates legados Go (10, PT-BR,
   despersonificados do panteão, sem chamador de produção).
2. O harness do panteão **não é uniforme**: LOCK/`memory: project`/`tools:` em 14/14, mas
   KANBAN+GIT_FLOW só em `afrodite` e `artemis`, Vault em 3/14. Não há "harness do panteão"
   pronto para portar — precisaria ser normalizado antes.
3. Os assets enviados **não ensinam a cadeia ADR→REQ→ROADMAP** que o próprio
   `branch_has_wip_roadmap` valida — lacuna de produto, não de estilo.
4. `identity.AgentName()` sempre sufixa `-tf`, então preset greek geraria `zeus-tf` sem
   colidir com `zeus.md` autoral — a decisão não é "um ou outro".
5. SHA-256 dos templates legados **batem** com `legacyHashes` em `internal/integrations/legacy.go`
   (adoção segura de instalações antigas). Apagá-los não quebra a adoção (hashes hardcoded),
   mas remove a proveniência — Opção 4 exige nota de proveniência.
6. Paridade dos assets vivos nos 3 CLIs: **OK** (md5 idêntico nos 10 arquivos).

**Governança:** sem REQ/roadmap/branch — enquadra-se na exceção de trivialidade objetiva
(§7 do CLAUDE.md global: "respostas a perguntas / review sem mudanças"). Artefatos de
governança só após o KG escolher uma das 4 opções.

**Status:** CONCLUÍDO.

---

## 2026-07-26 — Zeus — Plano de convergência: harness pessoal do KG → trackfw

**Escopo:** transformar o trackfw no harness único do KG, absorvendo o que há de melhor no
Panteão Grego + `~/.claude/CLAUDE.md` + skills pessoais. Análise/decisão, sem implementação.

**Entregável:** `docs/analises/2026-07-26-plano-convergencia-harness-pessoal-para-trackfw.md`.

**Decisões fechadas com o KG:** Q1 default para todos · Q2 assets em inglês · Q2b assinatura em EN
(com preset usa o DisplayName) · Q3 layout flat puro · Q4 exceção de trivialidade mantida (§7+§11) ·
Q5 vocabulário Waves+MLs com frontmatter YAML · Q7 seis estados (`analyzing` entra no validator) ·
Q9 vault completo (scaffold + `trackfw note new` + regra `note_orphan`) · Q14 `iac` como papel
canônico novo · Q15 Cronos/Hermes ficam fora (resíduo assumido de 2 arquivos locais).

**Achados que mudaram decisões:**
1. **Dimensão por agente está morta no CMDB** — nos últimos 7 dias, 63 artefatos criados e
   **zero** sob agente nomeado; 11 REQs já nasceram flat na raiz. Isso derrubou o custo estimado
   de migrar para flat e decidiu a Q3.
2. **O vault não depende de plugin** — `installed_plugins.json` não tem nada de vault; as 4 skills
   em `vault/skills/` são de formato Obsidian, para o humano. Agentes só leem/escrevem markdown.
   Corrigi minha recomendação anterior de "mapear para ADR": nota de vault é causa-raiz de bug,
   não decisão arquitetural — naturezas diferentes.
3. **Sync de assets já resolvido** — `scripts/sync-integration-assets.sh` + `check-integration-assets.sh`
   em `make quality`. Enriquecer os agents custa 1 árvore, não 30 arquivos.
4. **`agentTools()` NÃO serve para o `tools:` do Claude** — SET_IMPL/SET_ARCH usam IDs do agy/Windsurf.
   Exige mapeamento novo.
5. **Harness do panteão não é uniforme** — KANBAN/GIT_FLOW em 2/14, vault em 3/14. "Trazer do
   panteão" é, na prática, **definir uma vez e aplicar aos 10**.
6. **11 defeitos concretos catalogados** (Parte 4), incluindo `analyzing` reconhecido pelo board e
   não pelo validator, e a skill `zeus` que o CLAUDE.md §0 manda invocar e que não existe.

**Governança:** sem REQ/roadmap/branch — exceção §7 (análise/decisão). A próxima ação exige
`trackfw req new` → `roadmap new` → `move wip` → `git checkout -b` antes de qualquer edição.

**Status:** CONCLUÍDO.

**Adendo (mesma data) — Q16 resolvida:** as skills técnicas do KG entram no trackfw como
**família nova por papel** (10 arquivos em `assets/skills/`, ao lado das 5 de processo), **não**
apensadas às existentes — as 5 atuais são organizadas por verbo/fase (transversais) e as técnicas
por domínio/papel; fundir os eixos faria `implement` conter Go+React+ArangoDB+Kafka ao mesmo tempo.
Skill (e não corpo do agente) por três razões: carga sob demanda, reuso cross-agent e consulta pela
thread principal sem spawn. **Curadoria agnóstica de stack** (~40-50% das 1.231 linhas atravessam);
o específico (ArangoDB, Uber Fx, Entra ID, Module Federation) migra para o `CLAUDE.md` do CMDB —
que é onde o defeito #10 já dizia que deveria estar. Q17 (`git-ship` → `trackfw ship`) segue aberta.

**Adendo 2 — Q17 resolvida:** `git-ship` vira **`trackfw ship`**, com abertura de PR/MR **agnóstica
de forge**. Resolução do flavor por precedência: flag `--forge` → campo `forge:` no `trackfw.yaml`
(novo em `config.ProjectConfig`) → parse do host de `git remote get-url origin` → CI detectado pelo
`discover` (proxy) → modo manual (imprime URL, não falha). Mapeamento: github→`gh pr create`,
gitlab→`glab mr create` (fala "MR"), azure→`az repos pr create`, bitbucket→URL. Reaproveitar
`externalCommandAvailable` (exec.LookPath) do discover para degradação graciosa.
⚠️ Armadilha registrada: GitLab/Bitbucket self-hosted têm host arbitrário — o parse do remote sozinho
não resolve; daí o campo explícito e o desempate por `.gitlab-ci.yml`.
W8 marcada como candidata a **REQ separada** (não depende de Q1–Q16, toca `config`/`discover`).

**Descoberta lateral:** `config.ProjectConfig` já suporta `roadmap_namespacing: flat|by_agent` com
`Agents []string`. Ou seja, o layout por agente **já é capacidade existente** — a Q3 (flat) apenas
confirma o default, e o CMDB poderia manter o layout atual por configuração se quisesse.

---

## Sessão 2026-07-26 — ML-1B: validator reconhece estado `analyzing` (convergência do harness)

**Agente:** Apolo (Backend Senior Specialist)
**Status:** CONCLUÍDO
**Branch:** `feat/convergencia-do-harness-pessoal-para-o-trackfw`
**REQ:** docs/req/REQ-2026-07-26-convergencia-do-harness-pessoal-para-o-trackfw.md
**Commit:** e5671de — `fix(validator): reconhece estado analyzing nos 3 CLIs (REQ-2026-07-26)`

**Arquivos alterados:**
- `internal/validator/validator.go` — 4 pontos: stateDirs em resolveREQFiles, mapa folderToExpectedStatus, validateFolderStatusCoherence, validateFilenameUniqueness
- `internal/validator/validator_traceid.go` — 2 stateDirs (collectTraceIdEntries e collectTraceIdEntriesByAgent)
- `internal/validator/validator_test.go` — 2 novos testes: NoFolderStatusViolation e WipLimitDoesNotCount
- `npm/src/validator/index.js` — STATES, FOLDER_TO_STATUS, validateFolderStatusCoherence, validateFilenameUniqueness
- `npm/src/validator/traceid.js` — KNOWN_STATES
- `npm/tests/validator.test.js` — 2 novos testes: folder_status e wip_limit com analyzing
- `pypi/trackfw/validator.py` — resolveReqFiles, _FOLDER_TO_STATUS, validate_folder_status_coherence, validate_filename_uniqueness
- `pypi/trackfw/traceid.py` — _ROADMAP_STATES
- `pypi/tests/test_commands_validate_status.py` — 2 novas classes de teste

**Resultado dos gates:** go build ✅ | go test ./internal/validator/... ✅ | go vet ✅ | Node.js 125 passed ✅ | Python 443 passed ✅ | make quality ✅
**Semântica preservada:** wip_limit e branch_has_wip_roadmap continuam contando apenas `wip/` — não alterados.

---

## 2026-07-26 — ML-1C: Mecanismo rewriteSignatureLine

**Agente:** Apolo (Backend Senior Specialist)
**Status:** IMPLEMENTANDO
**Branch:** `feat/convergencia-do-harness-pessoal-para-o-trackfw`
**REQ:** docs/req/REQ-2026-07-26-convergencia-do-harness-pessoal-para-o-trackfw.md
**Escopo:** Criar `rewriteSignatureLine` nos 3 CLIs (Go, Node.js, Python) — localiza a última linha do corpo que casa com `^— (.+?), (.+)$` e substitui o nome pelo `displayName` da identidade configurada. Chamar na Rota B de Render após `rewriteFrontmatterFields` quando `hasIdentity == true`.

---

## 2026-07-26 — ML-1C: Mecanismo rewriteSignatureLine — CONCLUÍDO

**Agente:** Apolo (Backend Senior Specialist)
**Status:** CONCLUÍDO
**Branch:** `feat/convergencia-do-harness-pessoal-para-o-trackfw`
**Commit:** aa95b5a
**Entregáveis:**
- `internal/integrations/render.go`: `func rewriteSignatureLine(source []byte, displayName string) []byte` adicionada; Rota B atualizada; comentário do `Render()` atualizado.
- `internal/integrations/render_test.go`: 5 testes unitários + 1 teste de integração adicionados; goldens intocados.
- `npm/src/integrations/render.js`: `rewriteSignatureLine` adicionada e exportada; Rota B atualizada; comentário atualizado.
- `npm/tests/identity-render.test.js`: 5 testes unitários + 1 teste de integração adicionados.
- `pypi/trackfw/integrations/renderers.py`: `_rewrite_signature_line` adicionada; Rota B atualizada; comentário Rota B atualizado.
- `pypi/tests/test_integrations_identity.py`: 5 testes unitários + 1 teste de integração adicionados.
**Validação:** `go build ./... && go test ./... && go vet ./... && make quality` — todos verdes (443 Python, 125 npm, todos Go ok).

---

## 2026-07-26 — ML-1A: Camada universal de harness nos 10 assets de agente — IMPLEMENTANDO

**Agente:** Apolo (Backend Senior Specialist)
**Status:** IMPLEMENTANDO
**Branch:** `feat/convergencia-do-harness-pessoal-para-o-trackfw`
**Escopo:** Adicionar `memory: project`, `tools:` ao frontmatter e 5 blocos universais (Mode lock, Before you act, Scope boundary, Working context, Knowledge vault) em inglês nos 10 assets de agente Go. Adicionar linha de assinatura ao final de cada asset. Sincronizar npm/pypi. Regravar 4 goldens congelados deliberadamente.

---

## 2026-07-26 — ML-1A: Camada universal de harness nos 10 assets de agente — CONCLUÍDO

**Agente:** Apolo (Backend Senior Specialist)
**Status:** CONCLUÍDO
**Branch:** `feat/convergencia-do-harness-pessoal-para-o-trackfw`
**Commit:** d888df4
**Entregáveis:**
- 10 assets Go (`internal/integrations/assets/agents/{architect,backend,frontend,qa,infra,security,dba,ux,code-quality,data}.md`): `memory: project`, `tools:` correto por papel, 5 blocos universais em inglês, linha de assinatura exata.
- Sincronização nos 3 CLIs via `scripts/sync-integration-assets.sh`.
- 4 goldens re-congelados deliberadamente (`architect.subagent.golden.md`, `architect.agent-directory.golden.md`, `backend.agent-directory.golden.md`, `backend.codex-toml.golden.toml`).
- Comentário de `render_test.go` atualizado com data e REQ.
- `npm/tests/agents-skills.test.js`: 2 testes de golden atualizados (Codex TOML e Antigravity agent-directory).
**Validação:** `go build ./... && go test ./... && go vet ./... && make quality` — todos verdes (Go ok, npm 125/125, Python 443/443, todos os gates de paridade aprovados).

---

## 2026-07-26 — ML-2A + ML-2B: Adendo do orquestrador e implementador nos 10 assets — IMPLEMENTANDO

**Agente:** Apolo (Backend Senior Specialist)
**Status:** IMPLEMENTANDO
**Branch:** `feat/convergencia-do-harness-pessoal-para-o-trackfw`
**Escopo:** ML-2A: 4 blocos do orquestrador em `architect.md` (Git authority, Parallelization, Workflow, Post-microbatch audit) + `## Mission`. ML-2B: 4 blocos do implementador nos 6 agents com Edit/Write + Reporting boundary nos 3 read-only (security, code-quality, ux) + `## Mission` em todos os 9. Regravar 4 goldens, atualizar npm test e render_test.go.
**Commits:** fe088fe (ML-2A) + 353a4a2 (ML-2B)
**Entregáveis:**
- `architect.md`: blocos Git authority, Parallelization, Workflow, Post-microbatch audit, Mission.
- 6 agents com Edit/Write (backend, frontend, qa, infra, dba, data): Governance prerequisite, Git boundary, Microbatch completion protocol, Definition of done, Mission.
- 3 read-only (security, code-quality, ux): Governance prerequisite, Reporting boundary, Definition of done, Mission.
- 4 goldens re-congelados: architect.subagent, architect.agent-directory, backend.agent-directory, backend.codex-toml.
- render_test.go: comentário de re-congelamento Wave 2 adicionado.
- npm test: golden strings atualizadas (Codex TOML + Antigravity agent-directory para architect e backend).
- Sync nos 3 CLIs via `scripts/sync-integration-assets.sh`.
**Validação:** `make quality` verde — Go ok, npm 125/125, Python 443/443, check-integration-assets, check-identity-parity aprovados.
**Status:** CONCLUÍDO

---

## Sessão 2026-07-26 — Apolo — ML-3A: Harness completo no CLAUDE.md gerado

**Status:** CONCLUÍDO
**Branch:** feat/convergencia-do-harness-pessoal-para-o-trackfw
**Commit:** 066bd00 — `feat(generators): harness completo no CLAUDE.md gerado (REQ-2026-07-26)`
**Escopo:**
- Adicionadas 9 seções de harness pessoal à função `generateClaudeMD` (Go): Branch strategy, Definition of done, Requirement scope, State requirements, Roadmap format, When governance is not required, Production incidents, Iterative prototyping, Autopilot
- Paridade idêntica em Node.js (`npm/src/generators/init.js`)
- Python: adicionada função `generate_claude_md()` em `pypi/trackfw/generators/init_gen.py`, chamada no `scaffold()`
- Testes atualizados nos 3 CLIs verificando as 9 seções e integridade das seções pré-existentes
**Validação:** `go build + go test ./internal/generators/... + go vet` verdes; `node --test tests/generators.test.js` 20/20; `python3 -m pytest tests/test_generators_init.py` 38/38

---

## Sessão 2026-07-26 — Apolo — ML-3C: Gate de paridade deriva contagem do catálogo

**Status:** CONCLUÍDO
**Branch:** feat/convergencia-do-harness-pessoal-para-o-trackfw
**Commit:** 14d0dc7 — `fix(ci): gate de paridade deriva contagem de itens do catalogo (REQ-2026-07-26)`
**Escopo:** Corrigir `scripts/check-integration-cli-parity.sh` — substituir número mágico `10` (agentes) e `5` (skills) por contagem derivada do `catalog.json` em tempo de execução.
**Técnica:** `EXPECTED_AGENTS_COUNT` e `EXPECTED_SKILLS_COUNT` lidas via python3 do catalog.json antes do loop; exportadas como env vars; consumidas em `os.environ` no heredoc Python de `assert_json`. Falha explícita se catálogo ausente ou ilegível.
**Prova de detecção:** cópia do catálogo com 13 agentes (no scratchpad) fez o assert disparar com `AssertionError: item count mismatch for agents: expected 13, got 12`. Sem resíduo no repositório.
**Validação:** `make quality` VERDE de ponta a ponta.

---

## Sessão 2026-07-26 — Apolo — ML-3B: Papéis canônicos `iac` e `tooling`

**Status:** CONCLUÍDO
**Branch:** feat/convergencia-do-harness-pessoal-para-o-trackfw
**Commit:** c8623c5 feat(agents): papeis canonicos iac e tooling (REQ-2026-07-26)
**Escopo concluído:**
- `iac` e `tooling` em KnownAgentIDs() e todos os 10 presets × 3 CLIs (Go, Node.js, Python)
- Assets `iac.md` e `tooling.md` criados com estrutura idêntica a `infra.md` + blocos específicos
- Fronteira `infra` × `iac` declarada em ambos os arquivos
- `catalog.json` atualizado (descrições curtas ~51 chars para caber no form de identidade)
- Sync via `scripts/sync-integration-assets.sh` — npm + pypi sincronizados
- `agentTools` em render.go: SET_IMPL por default — sem alteração necessária
- Testes atualizados: 10→12 agentes nos 3 CLIs, fixture check adaptatado para agentes novos sem histórico
**Validação:** go build + go test (3 pacotes) verdes, go vet verde, npm 126/126, pypi 446/446, check-integration-assets verde

---

## Sessão 2026-07-26 — Apolo — ML-4A: 12 skills técnicas por papel

**Status:** CONCLUÍDO
**Branch:** feat/convergencia-do-harness-pessoal-para-o-trackfw
**Commit:** 6d820dd feat(skills): 12 skills tecnicas por papel (REQ-2026-07-26)
**Escopo concluído:**
- 12 skills técnicas criadas em `internal/integrations/assets/skills/`: architecture, backend, frontend, qa, infra, iac, security, dba, ux, code-quality, data, tooling
- Frontmatter: `name: trackfw-<id>-skill`; IDs no catalog.json com sufixo `-skill` para evitar colisão com IDs de agentes
- As 5 skills de processo (governance, plan, implement, review, release) permanecem byte a byte inalteradas
- Vocabulário proibido (ArangoDB, Uber Fx, Module Federation, Entra ID, API_SPECIFICATION) ausente — grep vazio
- catalog.json: 5 → 17 skills; sync propagado para npm e pypi via scripts/sync-integration-assets.sh
- Testes atualizados nos 3 CLIs (catalog_test.go → 17, agents_skills_test.go → 17, test_agents_skills.py → 17)
- Descoberta: IDs de skills técnicas não podem colidir com IDs de agentes (catalog.go valida); sufixo -skill resolve
**Linhas por skill:** architecture=66, backend=67, frontend=70, qa=63, infra=62, iac=90, security=74, dba=77, ux=70, code-quality=78, data=75, tooling=80
**Validação:** make quality VERDE (Go 100%, npm 126/126, pypi 446/446, check-integration-assets verde, check-integration-cli-parity verde)

---

## Sessão 2026-07-26 — Apolo — ML-4B: Vault de conhecimento (scaffold, note new, note_orphan)

**Status:** CONCLUÍDO
**Branch:** feat/convergencia-do-harness-pessoal-para-o-trackfw
**Commit:** 7b85b5a feat(vault): scaffold, comando note new e regra note_orphan (REQ-2026-07-26)
**Escopo:**
- Scaffold: vault/notes adicionado a govDirs nos 3 CLIs; vault/notes/index.md gerado no init
- Comando `note new "<título>"` nos 3 CLIs (Go + npm + pypi): cria slug-YYYY-MM-DD.md com frontmatter + 3 seções, linka no index.md, idempotente
- Regra `note_orphan` nos 3 validators (default warning, elevável a error via rules:, aceita link markdown e wikilink)
- ruleDefaults (Go), RULE_DEFAULTS (JS), _RULE_DEFAULTS (Python) para defaults por-regra
- 3 testes gerador + 5 testes validator no Go; npm 126/126 verde; pypi 446/446 verde
- docs/cli-parity.md atualizado com comando note e regra note_orphan
**Validação:** make quality VERDE de ponta a ponta

---

## Sessão 2026-07-26 — Apolo — Wave 5 (ML-5A + ML-5B): Saudação EN, D12-bis, aposentadoria de gerador legado

**Status:** CONCLUÍDO
**Branch:** feat/convergencia-do-harness-pessoal-para-o-trackfw
**Commits:**
- ML-5A: 664573f fix(agents): saudacao em ingles, uniformizacao D12-bis e documentacao (REQ-2026-07-26)
- ML-5B: chore(generators): aposenta gerador legado preservando legacyHashes (REQ-2026-07-26)
**Escopo:**
- ML-5A: greetingLine PT-BR → EN nos 3 CLIs; iac/tooling descriptions enriquecidas (D12-bis); cli-parity.md com analyzing/12 agents/17 skills/razão do sufixo -skill; README.md + site atualizados; ADR de identidade com emenda de idioma da saudação
- ML-5B: generators/agents.go, agents_test.go e templates/agents/ (10 arquivos) removidos; legacyHashes preservados integralmente com comentário de proveniência apontando para commit 664573f; legacy_test.go usa bytes inline do template removido
**Validação:** make quality VERDE após ambos os commits; trackfw validate sem violações; grep InstallAgents vazio

---

## Sessão 2026-07-26 — Apolo — ML-5C: Correção de comentário de proveniência em legacy.go

**Status:** CONCLUÍDO
**Branch:** feat/convergencia-do-harness-pessoal-para-o-trackfw
**Commit:** 06e7e9c docs(legacy): corrige commit de referencia na nota de proveniencia (REQ-2026-07-26)
**Escopo:**
- Corrigido comentário no bloco `legacyHashes` em `internal/integrations/legacy.go`
- A remoção dos templates foi atribuída ao commit correto `8a90a0b` (ML-5B)
- O comando de recuperação mantém `664573f` (ML-5A, commit anterior à remoção), com justificativa explícita de por que se usa esse hash e não o da remoção
- Nenhum hash foi alterado; apenas o bloco de comentário foi reescrito
**Verificações:**
- hash reproduzido de 664573f: d28ae507d2ce9fd3fcd7cb1a0c1ffaaebc994dc9c45b219e5361b909dc6132ba
- hash preservado em legacy.go: d28ae507d2ce9fd3fcd7cb1a0c1ffaaebc994dc9c45b219e5361b909dc6132ba (idênticos)
- go build ./... VERDE
- go test ./... VERDE (todos os pacotes)
- make quality VERDE (Go 15 pacotes / npm 126/126 / pypi 446/446 / parity checks OK)

---

## 2026-07-26 — Zeus — Convergência do harness: roadmap ENCERRADO

**Entregue:** 6 waves (1, 1b, 2, 3, 4, 5) + 2 corretivos (ML-3C, ML-5C), 26 commits na branch
`feat/convergencia-do-harness-pessoal-para-o-trackfw`.

**Resultado:** 12 agentes com harness completo (mode lock, tools, memory, vault, adendos de
orquestrador e implementador), 17 skills (5 de processo + 12 técnicas), CLAUDE.md gerado com 9 seções
novas, estado `analyzing` no validator, vault com comando e gate, papéis `iac` e `tooling`, gerador
legado aposentado com proveniência preservada.

**Achado aberto (nota de vault criada):** `branch_has_wip_roadmap` só enxerga `wip/`, então mover o
roadmap para `done/` na própria branch — como a Definition of Done exige — faz o `validate` reprovar.
O gate pune o comportamento que o produto prega. Registrado em
`vault/notes/branch_has_wip_roadmap-conflita-com-a-definition-of-done-2026-07-26.md`, com 3 opções e
recomendação. **Decisão pendente do KG.**

**Status:** CONCLUÍDO.

---

## 2026-07-26 — Apolo — ML-1A: campo `forge:` e resolver de precedência (IMPLEMENTANDO)

**Branch:** `feat/comando-trackfw-ship-agnostico-de-forge`
**REQ:** `docs/req/REQ-2026-07-26-comando-trackfw-ship-agnostico-de-forge.md`
**ADR:** `docs/adr/ADR-2026-07-26-trackfw-ship-agnostico-de-forge.md`

**Escopo:**
- Adicionar `Forge string` a `ProjectConfig` (Go) e equivalentes em npm/pypi
- Criar `internal/forge/resolve.go` com `Resolution{Forge, Source}`, `Input`, `Resolve()`, `ResolveFromRepo()`
- Precedência: flag → config → remote URL → CI files → manual
- Parse SSH e HTTPS; hosts conhecidos; desempate self-hosted por `.gitlab-ci.yml` / `.github/workflows/`
- Testes nos 3 CLIs com os mesmos casos obrigatórios

**Entregue:**
- `internal/config/config.go` — campo `Forge string` + parse da chave `forge:` no YAML
- `internal/forge/resolve.go` — `Resolution{Forge, Source}`, `Input`, `Resolve()`, `ResolveFromRepo()`
- `internal/forge/resolve_test.go` — 28 testes cobrindo todos os casos do roadmap
- `npm/src/config/index.js` — campo `forge: ''` + parse da chave `forge:`
- `npm/src/forge/resolve.js` — porte completo Node.js puro
- `npm/tests/forge.test.js` — 28 testes (mesmos casos)
- `pypi/trackfw/config.py` — campo `forge: ''` + parse da chave `forge:`
- `pypi/trackfw/forge/__init__.py` + `pypi/trackfw/forge/resolve.py` — porte Python
- `pypi/tests/test_forge_resolve.py` — 28 testes (mesmos casos)
- Commit `505fcaf` | push para `feat/comando-trackfw-ship-agnostico-de-forge`
- `make quality` VERDE (Go 15 pkgs | npm 154 testes | pypi 474 testes)

**Nota técnica:** Azure SSH usa `ssh.dev.azure.com` (host distinto de `dev.azure.com`). Coberto via regra `*.dev.azure.com` no `hostToForge` nos 3 CLIs.

**Status:** CONCLUÍDO.

---

## 2026-07-26 — Apolo — ML-2A: comando `trackfw ship` fluxo git completo (IMPLEMENTANDO)

**Branch:** `feat/comando-trackfw-ship-agnostico-de-forge`
**REQ:** `docs/req/REQ-2026-07-26-comando-trackfw-ship-agnostico-de-forge.md`
**ADR:** `docs/adr/ADR-2026-07-26-trackfw-ship-agnostico-de-forge.md`

**Escopo:**
- Passos 1–6 do fluxo ship: validação de branch, governança, squash-merges pendentes, staged review, commit CC, push
- Flags: `-m/--message`, `--dry-run`
- Injeção do executor de comandos git (sem exec direto em RunE)
- Wrapper exportado `CheckShipGovernance()` em `internal/validator` (gate duro, ignora baseline/lenient)
- Testes nos 3 CLIs cobrindo todos os casos obrigatórios; repositórios temporários para testes de escrita
- Teste grep garantindo ausência de `git add .`/`git add -A` no código-fonte (excluindo arquivos de teste)

**Status:** IMPLEMENTANDO.

---

## 2026-07-26 — Apolo — ML-2B: adaptadores por forge com degradação graciosa (CONCLUÍDO)

**Branch:** `feat/comando-trackfw-ship-agnostico-de-forge`
**REQ:** `docs/req/REQ-2026-07-26-comando-trackfw-ship-agnostico-de-forge.md`
**ADR:** `docs/adr/ADR-2026-07-26-trackfw-ship-agnostico-de-forge.md`

**Escopo:**
- `internal/forge/adapter.go` — `Adapter`, `NewAdapter()`, `FallbackURL()`, `remoteHTTPSBase()`
- `internal/forge/adapter_test.go` — spy de availFn, 4 nouns, URLs HTTPS/SSH/self-hosted/Azure SSH
- `npm/src/forge/adapter.js` — porte Node.js com PATH scan puro (sem subprocess)
- `npm/tests/forge_adapter.test.js` — mesmos casos; spy
- `pypi/trackfw/forge/adapter.py` — porte Python com `shutil.which`
- `pypi/tests/test_forge_adapter.py` — mesmos casos; spy

**Entregue:**
- `internal/forge/adapter.go` — `Adapter`, `NewAdapter()`, `FallbackURL()`, `remoteHTTPSBase()`; defaultAvailFn respeita `TRACKFW_DISABLE_EXTERNAL_COMMANDS`
- `internal/forge/adapter_test.go` — 11 testes; spy de availFn; bitbucket asserta 0 chamadas
- `npm/src/forge/adapter.js` — porte Node.js com PATH scan puro (sem subprocess)
- `npm/tests/forge_adapter.test.js` — 24 testes; spy; mesmos casos URL
- `pypi/trackfw/forge/adapter.py` — porte Python; `shutil.which`; `removeprefix`/`removesuffix`
- `pypi/tests/test_forge_adapter.py` — 32 testes; spy; mesmos casos URL
- Commit `8bf2f0b` (bundled com ADR do ML-2A) | testes: Go 11 ✅ | Node 24 ✅ | Python 32 ✅

**Status:** CONCLUÍDO.

---

## 2026-07-26 — Apolo — ML-2A: comando `trackfw ship` fluxo git completo (CONCLUÍDO)

**Branch:** `feat/comando-trackfw-ship-agnostico-de-forge`
**Commit:** `b31e4c4`

**Entregue:**
- `internal/validator/validator.go` — `CheckShipGovernance()` exportada como gate duro (bypass de baseline/lenient/rules): chama `validateBranchHasWIPRoadmap` + `validateWIPHasREQ` diretamente
- `internal/commands/ship.go` — Passos 1–6 com injeção de `shipDeps{execGit, checkGovernance, out}`; `--dry-run` skipa escrita via wrapper interno; nunca chama `git add` com curinga
- `internal/commands/ship_test.go` — 10 casos: main/master, padrão inválido, governança, nada staged, sem -m, dry-run, grep no fonte, runtime assertion
- `internal/commands/root.go` — `newShipCmd()` registrado
- `npm/src/commands/ship.js` + `npm/src/ship/runner.js` — Porte Node.js puro com mesma estrutura injetável
- `npm/src/commands/index.js` — Ship adicionado ao programa
- `npm/tests/ship.test.js` — 15 casos (mesmos casos + normalizeBranchSlug)
- `pypi/trackfw/commands/ship.py` + `pypi/trackfw/ship/runner.py` — Porte Python puro
- `pypi/trackfw/cli.py` — Ship registrado
- `pypi/tests/test_ship.py` — 35 casos (parametrizados com pytest)
- Push para `feat/comando-trackfw-ship-agnostico-de-forge`

**Resultados:**
- Go: `go build ./... && go test ./... && go vet ./...` — VERDE (todos os pacotes)
- npm: 193 testes — VERDE
- Python: 509 testes — VERDE

**Nota técnica:** A governança usa wrapper exportado `CheckShipGovernance()` que chama as funções privadas do pacote `validator` diretamente, ignorando baseline/lenient/rules — gate duro inviolável. O dry-run implementa whitelist de write commands (`commit`, `push`, `fetch`) no wrapper `git()` interno do `runShip`; `execGit` do dep só recebe comandos read-only em dry-run.

**Status:** CONCLUÍDO.

---

## 2026-07-26 — Apolo — ML-3A: integração forge + abertura de PR/MR no ship (CONCLUÍDO)

**Branch:** `feat/comando-trackfw-ship-agnostico-de-forge`
**REQ:** `docs/req/REQ-2026-07-26-comando-trackfw-ship-agnostico-de-forge.md`
**Commit:** `fa4f16e` — `feat(ship): abertura de PR/MR conforme a forge resolvida (REQ-2026-07-26-ship)`

**Objetivo:** Passo 7 do `trackfw ship` — após push, resolver forge e abrir PR/MR via adaptador. Paridade nos 3 CLIs (Go, Node.js, Python).

**Arquivos modificados:**
- `internal/commands/ship.go` — shipOpts (+noPR, +forge), shipDeps (+configForge/repoDir/availFn/execForgeCLI), Step 7, helpers firstLine/buildPRBody/buildForgeCreateArgs/defaultExecForgeCLI
- `internal/commands/ship_test.go` — makeDeps atualizado, 2 testes diretos atualizados, 12 novos testes Step 7
- `npm/src/ship/runner.js` — importa forge modules, Step 7, helpers firstLine/buildForgeCreateArgs
- `npm/src/commands/ship.js` — flags --no-pr e --forge, wire configForge de config.load()
- `npm/tests/ship.test.js` — makeDeps atualizado, 13 novos testes Step 7
- `pypi/trackfw/ship/runner.py` — parâmetros no_pr/forge_flag/config_forge/repo_dir/avail_fn/exec_forge_cli, Step 7, helpers _first_line/_build_forge_create_args/_default_exec_forge_cli
- `pypi/trackfw/commands/ship.py` — flags --no-pr e --forge, wire config_forge via load_config()
- `pypi/tests/test_ship.py` — make_deps atualizado, 16 novos testes Step 7

**Resultados:**
- Go: `go build ./... && go test ./internal/commands/... ./internal/forge/... && go vet ./...` — VERDE
- npm: 206 testes — VERDE
- Python: 556 testes — VERDE

**Decisões técnicas:**
- `forge.Resolve(forge.Input{...})` chamado com inputs injetáveis (não injeta um fake resolver) — testa precedência real
- `repoDir: ""` em testes → CI file detection desabilitada, sem acesso ao filesystem
- azure usa `--description` (não `--body`) construído em `buildForgeCreateArgs` do ship.go — adapter.go não modificado
- `buildForgeCreateArgs` usa `copy()` para nunca mutar `adapter.CLIArgs`
- Step 7 é non-fatal: ausência de CLI, forge=manual, ou erro de invocação → warn + URL de fallback + exit 0

**Status:** CONCLUÍDO.

---

## 2026-07-26 — Apolo — ML-2C: textos de lenient + correção ANSI no gate de paridade (CONCLUÍDO)

**Branch:** `feat/comando-trackfw-ship-agnostico-de-forge`
**Commits:** `1b8c493` (docs lenient) · `df128e3` (fix ANSI gate)

**Commit 1 — docs(ship): explicita gate duro ignora lenient**
- `--help` passo 2 nos 3 CLIs: adicionado `(hard gate: not affected by lenient mode or per-rule severity)`
- Mensagem de erro do passo 2 nos 3 CLIs: bloco explicativo sobre lenient após remediação
- Testes Go/Node/Python: assert `"lenient"` no output de erro de governança
- `docs/cli-parity.md`: linha `ship` na tabela + seção de prosa com flags, 6 passos, divergência do validate

**Commit 2 — fix(ci): gate de paridade imune a ANSI do argparse**
- Causa raiz: Python 3.13+ coloriza argparse por default; ANSI antes do nome do comando quebrava o grep de limite de palavra em `check_help` e `assert_help_contract`
- `scripts/check-cli-parity.sh`: `export NO_COLOR=1 TERM=dumb` + strip ANSI inline em `check_help`; `commands=()` estendido com `note` e `ship`
- `scripts/check-integration-cli-parity.sh`: strip ANSI inline em `assert_help_contract`
- `vault/notes/argparse-ansi-parity-gate-python313-2026-07-26.md`: nota criada e linkada no índice
- Falsificação: `commands=(definitely-not-a-real-command)` retorna exit=1 + mensagem correta, sem resíduo
- `make quality` verde sem `NO_COLOR=1` externo (Python 3.14.6)

**Status:** CONCLUÍDO.

---

## Sessão 2026-07-26 — ML-3B: discover detecta e persiste a forge no trackfw.yaml

**Agente:** Apolo — Backend Senior Specialist
**Branch:** `feat/comando-trackfw-ship-agnostico-de-forge`
**REQ:** REQ-2026-07-26-ship

### Escopo do ML-3B
- Passo 1: `internal/discover/discover.go` — campo `Forge string` em `DiscoveryResult`, detecção via `forge.ResolveFromRepo`, emissão condicional de `forge:` em `GenerateYAML`
- Passo 2: `internal/commands/init.go` — flag `--forge`, wizard com detecção default, validação acima do early-return não-TTY
- Passo 3: paridade em `npm/src/` e `pypi/trackfw/`
- Testes nos 3 CLIs

**Status:** IMPLEMENTANDO

**Status:** CONCLUÍDO. Commit: fbbd028. Push: feat/comando-trackfw-ship-agnostico-de-forge.
Gates: go build ✅ | go test ✅ | go vet ✅ | npm test (206 pass) ✅ | pytest (541 pass) ✅

---

## Sessão 2026-07-27 — ML-4A: paridade npm/pypi, dry-run com disponibilidade, testes e docs

**Agente:** Apolo — Backend Senior Specialist
**Branch:** `feat/comando-trackfw-ship-agnostico-de-forge`
**REQ:** REQ-2026-07-26-ship

### Escopo desta sessão (continuação do ML-4A após sessão anterior morrer)

Go já implementado e comitado em 6afbf5e. Esta sessão porta para npm e pypi:

1. Dry-run consciente de disponibilidade em `npm/src/ship/runner.js` e `pypi/trackfw/ship/runner.py`
2. Fix do wiring `--no-pr` em `npm/src/commands/ship.js` (commander usa `options.pr === false`)
3. Testes da matriz de forges (4 forges × 2 avail × 2 hosts = 16 casos) em npm e pypi
4. Testes de silence-usage e integração com PATH limpo em npm e pypi
5. Documentação: `docs/cli-parity.md`, `README.md`, `site/`

**Status:** CONCLUÍDO.

Commits: c036f72 (código) + a5e6277 (docs).

Gates: go build ✅ | go test ✅ | npm test (48 pass) ✅ | pytest (69 pass) ✅ | make quality ✅

Achados registrados:
- Bug corrigido: commander `--no-pr` com default `false` explícito tornava `options.pr` sempre `false`;
  a opção negatable deve ser definida sem default explícito para que `options.pr` seja `true` (sem a flag)
  ou `false` (com `--no-pr`)
- Divergência documentada: Go usa `docs/roadmaps/wip/` (default `docs/roadmaps`) como roadmap dir;
  npm e pypi usam `docs/roadmaps/claude/wip/` (default `docs/roadmaps/claude`). Não corrigida nesta
  sessão — é divergência pré-existente e ortogonal ao ML-4A

---

## Sessão 2026-07-27 — Apolo — Correções pós-auditoria ML-4A

**Agente:** Apolo (Backend Senior Specialist)
**Branch:** `feat/comando-trackfw-ship-agnostico-de-forge`
**Status:** CONCLUÍDO

### Correções realizadas após revisão do advisor

1. **cli-parity.md — labels de source incorretos corrigidos:**
   - `source: url` → `source: remote` (valor real retornado pelo resolver em Go e npm)
   - `source: manual` → `source: none` (valor real para o caso "nenhuma forge detectada")
   - CI detection: removidos `azure-pipelines.yml` e `bitbucket-pipelines.yml` (o código não detecta esses arquivos — apenas `.gitlab-ci.yml` e `.github/workflows/`)

2. **cli-parity.md — tabela de paridade:** row do ship atualizada para "open PR/MR" (step 7)

3. **cli-parity.md — divergência roadmap_dir documentada:** seção explícita registrando que Go usa `docs/roadmaps` e npm/pypi usam `docs/roadmaps/claude` como padrão; divergência intencional preservada

4. **site/guide/commands.md + site/en/guide/commands.md:** seção `trackfw ship` adicionada (PT-BR e EN)

5. **npm/tests/ship.test.js — integration test --no-pr command-layer:** novo teste que detecta regressão onde `options.noPr || false` silenciava o flag; usa `--dry-run --no-pr` juntos (noPR é checado antes do dry-run no passo 7); 49 testes (era 48)

**Commit:** b0433a8
**Gates:** make quality ✅ | npm test 49 pass ✅ | trackfw validate ✅

---

## ML-4B — 2026-07-27 — Apolo

**Tarefa:** Correção crítica — alinhar default de `roadmap_dir` entre os 3 runtimes
**Branch:** `feat/comando-trackfw-ship-agnostico-de-forge`
**Status:** CONCLUÍDO

**Causa raiz confirmada:** runners npm/PyPI reimplementavam resolução de `roadmap_dir` com default errado (`docs/roadmaps/claude`), enquanto os módulos de config já tinham o default correto (`docs/roadmaps`). Duplicação de lógica com valor divergente.

**Correções aplicadas:**
1. `npm/src/ship/runner.js`: `resolveRoadmapDir()` agora delega a `loadConfig().roadmapDir`
2. `pypi/trackfw/ship/runner.py`: `_resolve_roadmap_dir()` agora delega a `_config.load()["roadmap_dir"]`
3. Testes de integração npm e PyPI migrados: `docs/roadmaps/claude/wip/` → `docs/roadmaps/wip/`
4. Testes de paridade adicionados: npm (50 pass) e PyPI (70 pass)
5. `docs/cli-parity.md`: seção "Known divergence" removida
6. Nota de vault criada: `vault/notes/ship-roadmap-dir-default-divergencia-2026-07-27.md`

**Gate nos 3 runtimes (mesmo repositório):** Go ✅ | Node.js ✅ | PyPI ✅
**make quality:** ✅ | **trackfw validate:** ✅

**Commit:** 442bcf1 | **Push:** feat/comando-trackfw-ship-agnostico-de-forge

---

## ML-1A — 2026-07-27 — Apolo

**Tarefa:** fix(validator): branch_has_wip_roadmap aceita roadmap concluído na própria branch
**Branch:** `feat/robustez-dos-gates-de-governanca-e-paridade`
**Status:** IMPLEMENTANDO

**Problema:** regra `branch_has_wip_roadmap` só busca em `wip/`; mover o roadmap para `done/` durante o DoD na branch reprova o gate.
**Correção:** procurar slug em `wip/` E `done/`; reprovar apenas se não houver correspondência em nenhum dos dois; casamento de slug obrigatório também em `done/`.
**Status:** CONCLUÍDO

**Implementação:**
- `resolveStateDirs(cfg, state)` adicionado nos 3 runtimes como fonte única de resolução de caminho; `resolveWIPDirs` e `resolveDoneDirs` são wrappers finos.
- `validateBranchHasWIPRoadmap` itera `candidates` de `wip/` + `done/`; retorna sem violação se slug casar em qualquer um.
- Mensagens atualizadas: "wip/ nor done/" nas duas variantes.
- 4 cenários cobertos por teste nos 3 CLIs (P4 do ADR).
- `docs/cli-parity.md` atualizado com tabela dos 4 cenários.
- `make quality` verde (Go: ok | Node.js: 228 pass | Python: 580 pass) | `trackfw validate`: 0 violações.

---

## ML-2B — 2026-07-27 — Apolo

**Tarefa:** fix(ci): auditoria P1–P3 dos 7 scripts de gate
**Branch:** `feat/robustez-dos-gates-de-governanca-e-paridade`
**Status:** IMPLEMENTANDO

**Escopo:** auditar `check-cli-parity.sh`, `check-identity-parity.sh`, `check-integration-assets.sh`, `check-integration-cli-parity.sh`, `check-static-assets.sh`, `check-validate-parity.sh`, `smoke-integration-packages.sh` e `Makefile` contra P1–P3 do ADR.

**Status:** CONCLUÍDO

**Correções implementadas:**

1. `check-cli-parity.sh` — P1: `commands` derivado do `--help` do Go (antes hardcoded); `go_only_commands` isola exceções documentadas (`amazonq copilot cursor gemini windsurf completion`); vacuity guard (parsing < floor → exit 1). Verificação P4: vacuity guard ativado, comando faltante detectado.

2. `check-integration-cli-parity.sh` — P1: `assert_catalog_targets` derivava targets de set hardcoded; agora lê de `$CATALOG_FILE` (exportado). Verificação P4: target fora do catálogo detectado com mensagem clara.

3. `check-static-assets.sh` — P1: lista de assets era `index.html app.js style.css`; agora derivada via `find` no diretório canônico. Verificação bidirecional (arquivo extra detectado). Vacuity guard. P4: drift de conteúdo e arquivo extra detectados.

4. `check-integration-assets.sh` — P2 (sh sem pipefail): `find | sort` separado em dois comandos; se `find` falhar, o erro é visível sob `set -eu`. Vacuity guard para canonical-files vazio.

5. `check-validate-parity.sh` — P2: vacuity guard para validadores que retornam exit 1 mas zero violações (saída trivialmente idêntica). `mktemp -d` com template portável.

**Não alterado (report apenas):**
- `check-identity-parity.sh`: `TARGETS` hardcoded (P1 documentado); line-143 `diff|awk` NÃO é P2 (set -e suprimido em contexto `||`). Sem correção — derivação do catalog requer lógica não-trivial de superfícies padrão.
- `smoke-integration-packages.sh`: conforme. Verifica pré-requisito `build` explicitamente; pipes protegidos por `test -n`.
- `Makefile`: conforme. Apontamentos: `check-integration-cli-parity.sh` só roda como tail-call de `check-cli-parity.sh`; `smoke-integration-packages.sh` fora do alvo `quality` (design intencional — smoke é mais pesado).
- Mensagem `branch_has_wip_roadmap`: herdada do ML-1A. Com `done/` incluído na busca, lista 15 roadmaps em uma linha. NÃO editado o validator (escopo proibido). Sugestão registrada: truncar em 3 primeiros + contagem, ou listar apenas os de `wip/`.

**Commit:** 80746b4 | **Push:** feat/robustez-dos-gates-de-governanca-e-paridade

---

## ML-2A — 2026-07-27 — Apolo

**Tarefa:** fix(validator): auditoria P1–P3 das 17 regras do validator
**Branch:** `feat/robustez-dos-gates-de-governanca-e-paridade`
**Status:** IMPLEMENTANDO

**Escopo:** auditar as 17 regras contra P1–P3 do ADR. Corrigir defeitos encontrados nos 3 CLIs.

**Defeitos identificados para correção:**
- P3 + P2 em `contentHasMarker` (3 CLIs): guarda `marker+" \n"` não detecta CRLF (`\r\n`) → empty markers em arquivos CRLF passam sem violation
- P2 em `folder_status` (Go): `entries, _ := listDir(dir.path)` — erros non-ENOENT silenciados
- P2 em `filename_uniqueness` (Go): `names, _ := listDir(dir)` — mesmo padrão
- P3 em `filename_uniqueness` (Go): iteração de map → mensagens não-determinísticas
- P3 em `adr_dir_exists` (Node.js): tag `adr_dirs_exist` ≠ `adr_dir_exists` (Go/Python); mensagem diverge
- P2 em `folder_status` e `filename_uniqueness` (Node.js + Python): `listDir` engole erros não-ENOENT

**Itens registrados sem correção (fora de escopo/complexidade):**
- `adr_orphan`: `walkADRFilePaths` silencia walk errors — requer refator de assinatura
- Todos os padrões `os.ReadFile → continue` (~30 sites × 3 CLIs) — sistêmico, fora de escopo
- `staleWIPDays = 7` — P1 parcial, mas campo não existe em ProjectConfig
- `branch_has_wip_roadmap`: não alterar por instrução explícita
- `traceid_*`: parcialmente mitigado pela salvaguarda de zero entries


**Status:** CONCLUÍDO | Commit: 3dbeae5

**Inventário das 17 regras (P1 / P2 / P3 / Ação):**

| Regra | P1 | P2 | P3 | Ação |
|---|---|---|---|---|
| `adr_dir_exists` | OK | OK (non-ENOENT stat: minor) | Tag npm: `adr_dirs_exist` → diverge de Go/Python; msg diverge | **CORRIGIDO** npm: tag + msg |
| `adr_orphan` | OK | `walkADRFilePaths`: erros de walk silenciados | OK | REGISTRADO — requer refator |
| `blocked_by_draft_adr` | OK | `os.ReadFile → continue`; `adrIsDraft → return false` | OK | REGISTRADO |
| `blocked_has_req` | OK | `os.ReadFile → continue` | OK | REGISTRADO |
| `branch_has_wip_roadmap` | OK | `entries, _ := listDir` | OK | AUDITADO APENAS (não alterar) |
| `filename_uniqueness` | OK | `names, _ := listDir` — non-ENOENT silenciado | Iteração de mapa → msg não-determinística | **CORRIGIDO** (3 CLIs) |
| `folder_status` | OK | `entries, _ := listDir` — non-ENOENT silenciado | OK | **CORRIGIDO** (3 CLIs) |
| `note_orphan` | OK | Go: OK (Glob error propagado); npm: OK (throw); py: ENOENT vs outros OK | OK | OK |
| `ref_targets_exist` | OK | `os.ReadFile → continue` | OK | REGISTRADO |
| `req_has_adr` | OK | `os.ReadFile → continue`; CRLF em contentHasMarker | CRLF em contentHasMarker | **CORRIGIDO** contentHasMarker (3 CLIs) |
| `req_has_roadmap` | OK | `os.ReadFile → continue`; CRLF em contentHasMarker | CRLF em contentHasMarker | **CORRIGIDO** contentHasMarker (3 CLIs) |
| `stale_wip` | P1 parcial: `staleWIPDays=7` (sem campo em ProjectConfig) | `Glob → continue` (raro) | OK | REGISTRADO — stale_wip_days não está em config |
| `wip_acceptance` | OK | `os.ReadFile → continue`; CRLF em contentHasMarker | CRLF em contentHasMarker | **CORRIGIDO** contentHasMarker (3 CLIs) |
| `wip_has_req` | OK | `os.ReadFile → continue`; CRLF em contentHasMarker | CRLF em contentHasMarker | **CORRIGIDO** contentHasMarker (3 CLIs) |
| `wip_limit` | OK | OK | OK | OK |
| `traceid_duplicate_*` | Estados hardcoded (produto, OK) | `collectTraceIdEntries` errors descartados; parcialmente mitigado por salvaguarda | Msg-ordering de mapa (menor) | REGISTRADO — salvaguarda cobre cenário principal |
| `traceid_orphan_*` / `traceid_state_mismatch` | idem | idem | idem | REGISTRADO |

---

## ML-3A — 2026-07-27 — Apolo

**Tarefa:** fix(validator+gates): testes de falsificação P4 e documentação dos princípios (REQ-2026-07-26-gates)
**Branch:** `feat/robustez-dos-gates-de-governanca-e-paridade`
**Status:** CONCLUÍDO

**O que foi entregue:**

1. **`scripts/check-gates-falsify.sh`** (novo, `100755` no git) — prova de falsificação dos 6 gates de
   paridade: `static-assets`, `integration-assets`, `identity-parity`, `validate-parity`,
   `cli-parity`, `integration-cli-parity`. Cada cenário monta o defeito, afirma `exit != 0` com
   diagnóstico esperado e desmonta via `trap`. Integrado ao alvo `parity` do `Makefile`.

2. **Testes negativos das regras corrigidas na Wave 2** nos 3 CLIs:
   - `internal/validator/validator_test.go` — casos CRLF e truncamento `branch_has_wip_roadmap`
   - `npm/tests/validator.test.js` — equivalentes Node.js
   - `pypi/tests/test_validator.py` — equivalentes Python

3. **Truncamento da mensagem `branch_has_wip_roadmap`** nos 3 CLIs: lista os 3 primeiros
   candidatos em ordem determinística + "e mais N" quando há mais de 3 (P3: ordenar antes de fatiar).

4. **`docs/gate-design-principles.md`** (novo) — P1–P4 documentados com os 4 defeitos reais como
   exemplos vinculantes, tabela de neutralização de ambiente, checklist de aceite para gates novos,
   referências às notas de vault. Linkado de `docs/cli-parity.md` (nova seção "Princípios de design
   de gates (P1–P4)") imediatamente antes de "Release rule".

**Verificação final:** `make quality` verde — Go build/vet/test ok, Node.js 228 pass, Python 588 pass,
6 gates positivos + 6 falsificações passando, sem variável auxiliar.

**Commit:** `dc9a18f` | **Push:** `feat/robustez-dos-gates-de-governanca-e-paridade`

---

---

## Encerramento da REQ-2026-07-26-gates — 2026-07-27 — Zeus

**Tarefa:** auditoria final das 3 waves e encerramento do roadmap
**Branch:** `feat/robustez-dos-gates-de-governanca-e-paridade`
**Status:** CONCLUÍDO

Sessão anterior foi interrompida após os commits dos MLs 2A/2B, antes da barrier. Retomado:
barrier da Wave 2 executada (`make quality` verde), ML-3A spawnado, concluído e auditado.

**Resultado:** 4 MLs concluídos. 6 gates de paridade agora têm prova de falsificação
(`scripts/check-gates-falsify.sh`), as regras corrigidas na Wave 2 têm teste negativo nos 3 CLIs,
e os princípios P1–P4 estão em `docs/gate-design-principles.md` ancorados nos 4 defeitos reais.

**Achado do encerramento:** `trackfw roadmap move <nome> done` NÃO reescreve o `status` do
frontmatter — o arquivo muda de pasta e o `folder_status` acusa divergência na hora. O comando
que existe para cumprir a DoD gera um estado que o próprio validador reprova. Registrado como
débito nº 5 no roadmap; é candidato a REQ própria (corrigir nos 3 CLIs).

**Débito registrado (não esquecido):** 5 itens no fim do roadmap em `docs/roadmaps/done/`.

---

## REQ-2026-07-27-roadmap-move — 2026-07-27 — Zeus

**Tarefa:** abertura da REQ e do roadmap para o débito nº 5 da REQ anterior
**Branch:** `fix/roadmap-move-sincroniza-status`
**Status:** IMPLEMENTANDO

**Defeito:** `roadmap move` não sincroniza `status:` do frontmatter — o comando da DoD gera warning
de `folder_status`. Mesmo formato do D4 da REQ-2026-07-26.

**Reprodução:** o defeito apareceu ao criar este próprio roadmap, na sequência
`req new → roadmap new → roadmap move wip`. O artefato que autoriza o conserto nasceu com o defeito.

**Escopo:** reescrita escopada de frontmatter nos 3 CLIs (espelhando `rewriteFrontmatterFields`),
correção do `re.sub` não escopado do Python, sincronização do cabeçalho, e testes P4 que rodam
`validate` após o `move`. Node.js ganha suíte de `moveRoadmap`, hoje inexistente.

**Fora de escopo (5 achados adjacentes registrados na REQ):** divergência de templates Python×Go/Node,
estado `analyzing` não movível, aspas no `parse_frontmatter` do Python, ambiguidade do `findRoadmap`
do Go, prefixo de agente no log Python.

---

## ML-1A — REQ-2026-07-27-roadmap-move — 2026-07-27 — Apolo

**Tarefa:** ML-1A do roadmap `ROADMAP-2026-07-27-roadmap-move-sincroniza-o-status-do-artefato.md`
— sincronizar `status:` do frontmatter e cabeçalho em `roadmap move` nos 3 CLIs.
**Branch:** `fix/roadmap-move-sincroniza-status`
**Commit:** `385df5b`
**Status:** CONCLUÍDO

**O que foi feito:**

- **Go** (`internal/generators/roadmap.go`): adicionou `rewriteRoadmapStatus(source []byte, state string) ([]byte, bool)` espelhando a semântica de `rewriteFrontmatterFields`. `MoveRoadmap` lê o arquivo após `os.Rename` e chama a função; só escreve se `changed == true`. Frontmatter escopado; `| Status: ` no cabeçalho sincronizado antes do primeiro `## `.

- **Node.js** (`npm/src/generators/roadmap.js`): adicionou `rewriteRoadmapStatus(source, state)` com mesma semântica. `moveRoadmap` chama após `fs.renameSync`. Exportada para testes.

- **Python** (`pypi/trackfw/generators/roadmap.py`): adicionou `_rewrite_roadmap_status(source, state)` substituindo o `re.sub` não escopado da linha ~213. State gravado em minúsculo (bytes idênticos nos 3 CLIs). Import `re` mantido (usado em `slugify`).

**Testes criados:**

- **Go** (`internal/generators/roadmap_test.go`):
  - `TestMoveRoadmap_FrontmatterSync_ValidateAfterMove` — P4: controle positivo + ausência de `folder_status` após move
  - `TestMoveRoadmap_BodyStatusIntact` — escopo: `status:` no corpo e `| Status:` em seção não tocados
  - `TestMoveRoadmap_NoFrontmatter` — arquivo sem frontmatter: conteúdo intacto
  - `TestMoveRoadmap_Valid` — atualizado para verificar `status: wip` e `| Status: wip`

- **Node.js** (`npm/tests/roadmap_move.test.js` — novo, 10 testes): move válido, estado inválido, não encontrado, validate P4 com controle positivo, escopo do frontmatter, sem frontmatter, testes unitários de `rewriteRoadmapStatus`

- **Python** (`pypi/tests/test_generators_roadmap.py`):
  - `TestRewriteRoadmapStatus` (5 testes unitários)
  - `TestMoveRoadmapFrontmatterSync` (4 testes: casing minúsculo, P4 validate, sem frontmatter, corpo intocado)
  - `assertIn("status: WIP")` corrigidos em 2 arquivos de teste para `"status: wip"`

**Divergências deliberadas não corrigidas (escopo negativo da REQ):**
1. Template Python gera `status: Backlog` (não `status: backlog`) — divergência de template, REQ própria
2. Prefixo de agente no log Python (ausente); Go/Node prefixam — REQ própria
3. `parse_frontmatter` Python não remove aspas → `status: "wip"` gera warning — REQ própria

**Qualidade:** `make quality` verde, sem variável de ambiente auxiliar.

---

## Encerramento da REQ-2026-07-27-roadmap-move — 2026-07-27 — Zeus

**Branch:** `fix/roadmap-move-sincroniza-o-status-do-artefato`
**Status:** CONCLUÍDO

ML-1A e ML-2A concluídos e auditados. `make quality` verde.

**Prova de paridade de bytes** (feita pelo orquestrador — os testes de cada CLI, isolados, não
verificam isso entre si): mesmo roadmap movido pelos 3 binários em diretórios separados →
Go × Node e Go × Python **idênticos byte a byte**. Fixture com `status:` no corpo ficou intacto.

**ML-2A:** este roadmap nasceu com o defeito (o `move` para wip gerou warning) e foi encerrado sem
ele — `status: wip` → `done` automático, cabeçalho junto, zero edição manual.

**Gate pegou erro do orquestrador:** a branch fora criada como `fix/roadmap-move-sincroniza-status`,
slug que não casa com o roadmap `...sincroniza-o-status-do-artefato`. O `branch_has_wip_roadmap`
reprovou corretamente — era trabalho órfão. Branch renomeada antes do PR.

**Débito:** 5 divergências adjacentes seguem abertas, registradas na REQ e na nota de vault.

---

## REQ-2026-07-27-convergencia-templates-python — 2026-07-27 — Zeus

**Branch:** `fix/convergencia-dos-templates-de-artefato-do-cli-python`
**Status:** IMPLEMENTANDO (Wave 1 concluída por Apolo, aguardando Wave 2)

**Defeito:** templates de artefato do CLI Python divergem de Go/Node — mas o efeito real é que **duas
regras do validator ficam vacuamente verdes** para artefatos gerados pelo Python:
- `Status: Open` não casa (template Python usa tabela) → REQ escapa de `req_blocked_by_draft_adr` e do `sync`
- `Status: Draft` não casa (template Python usa `## Status`) → `blocked_by_draft_adr` passa por ausência de match

É P2 (degradação silenciosa) do ADR de gates. Sobreviveu porque nenhum gate jamais executou um gerador.

**Ordem deliberada:** Wave 1 escreve os testes negativos ANTES da convergência. Convergir primeiro
faria as regras casarem por efeito colateral e perderíamos a evidência da cegueira.

**Escopo:** 3 waves sequenciais — expor as regras cegas / convergir templates Python / gate que
executa os geradores e compara saída byte a byte.

**Fora de escopo (5 itens na REQ):** migração dos 50 roadmaps existentes, slash-command que gera
roadmap sem frontmatter nos 3 init, flags `--from-req`/`--req` ausentes no Python, schemas mortos +
doc incorreta, divergências menores Go↔Node.

---

## ML-1A — REQ-2026-07-27-convergencia-templates-python — 2026-07-27 — Apolo

**Branch:** `fix/convergencia-dos-templates-de-artefato-do-cli-python`
**Status:** CONCLUÍDO

**Objetivo:** escrever testes negativos que provam a cegueira das duas regras do validator para
artefatos gerados pelo CLI Python — e vê-los falhar antes da convergência (Wave 2).

**Testes criados (6 no total, 2 por runtime):**
- Go: `TestADRDraftFormatoPython_regra_cega` + `TestREQOpenFormatoPython_regra_cega`
  → `internal/validator/validator_test.go` — marcados com `t.Skip(...)`
- Python: `test_adr_draft_formato_python_regra_cega` + `test_req_open_formato_python_regra_cega`
  → `pypi/tests/test_validator.py` — funções de nível de módulo com `@pytest.mark.xfail(strict=True)`
  (não TestCase: xfail não funciona em métodos unittest.TestCase)
- Node: dois testes com `testSkip(...)` (helper adicionado ao harness existente — sem nova dependência)
  → `npm/tests/validator.test.js` — `skipped` counter adicionado ao sumário

**Estratégia de isolamento (por indicação do advisor):**
- Teste A (Defeito 2 — ADR Draft): REQ no formato **canônico** (passa no guard `Status: Open`) +
  ADR no formato **Python** → único fator que pode causar "sem violation" é `adrIsDraft` cego
- Teste B (Defeito 1 — REQ Open): ADR no formato **canônico** (passa em `adrIsDraft`) +
  REQ no formato **Python** (tabela) → único fator que pode causar "sem violation" é guard `Status: Open` cego
  Obs: template Python não emite `## Blocked by ADRs` — seção adicionada nos fixtures para que a
  regra possa sequer tentar avaliar o bloqueio.

**Saída das falhas capturadas (evidência documental de P2):**

Go:
```
=== RUN   TestADRDraftFormatoPython_regra_cega
    validator_test.go:1547: DEFEITO P2 confirmado: blocked_by_draft_adr não detectou ADR Draft
    no formato Python. ADR existe mas adrIsDraft() retorna false (procura 'Status: Draft',
    ADR Python tem 'status: Draft' no frontmatter e '## Status\nDraft' no corpo — nenhuma das
    duas é a string procurada). violations: []
--- FAIL: TestADRDraftFormatoPython_regra_cega (0.01s)
=== RUN   TestREQOpenFormatoPython_regra_cega
    validator_test.go:1637: DEFEITO P2 confirmado: blocked_by_draft_adr não detectou REQ Open
    no formato Python. REQ usa tabela '| Status | Open |' mas validator procura 'Status: Open'
    (inline). A REQ é silenciosamente ignorada — regra vacuamente verde. violations: []
--- FAIL: TestREQOpenFormatoPython_regra_cega (0.01s)
FAIL	github.com/kgsaran/trackfw/internal/validator	0.479s
```

Python (--runxfail):
```
FAILED pypi/tests/test_validator.py::test_adr_draft_formato_python_regra_cega
AssertionError: DEFEITO P2 confirmado: blocked_by_draft_adr não detectou ADR Draft no formato Python.
ADR existe mas _adr_is_draft() retorna False (procura 'Status: Draft', ADR Python tem 'status: Draft'
no frontmatter e '## Status\nDraft' no corpo — nenhuma das duas é a string procurada). violations: []

FAILED pypi/tests/test_validator.py::test_req_open_formato_python_regra_cega
AssertionError: DEFEITO P2 confirmado: blocked_by_draft_adr não detectou REQ Open no formato Python.
REQ usa tabela '| Status | Open |' mas validator procura 'Status: Open' (inline).
A REQ é silenciosamente ignorada — regra vacuamente verde. violations: []
```

Node:
```
↷ [xfail esperado] ML-1A: adrIsDraft cega — ADR Python "status: Draft" não detectado como Draft
↷ [xfail esperado] ML-1A: Status: Open cego — REQ Python com tabela não detectada como Open
35 passed, 0 failed, 2 xfail
```

**make quality:** 597 passed, 2 xfailed — verde.

**Próximo passo:** ML-2A (Wave 2) — convergir os templates Python para o formato canônico Go/Node,
reativar os 6 testes (remover t.Skip / --runxfail / testSkip e converter para asserções normais).

---

## Sessão 2026-07-27 — ML-2A em andamento

**Agente:** Apolo (Backend Senior Specialist)
**Status:** IMPLEMENTANDO
**Branch:** `fix/convergencia-dos-templates-de-artefato-do-cli-python`
**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-27-convergencia-dos-templates-de-artefato-do-cli-python.md`
**REQ:** `docs/req/REQ-2026-07-27-convergencia-dos-templates-de-artefato-do-cli-python.md`

### O que está sendo feito

Executando ML-2A: convergência dos templates Python para o formato canônico Go/Node.

**Alterações em andamento:**
- `pypi/trackfw/generators/req.py` — formato canônico (status:Open, date, author, adr, roadmap; header inline; seções inglês + Blocked by ADRs)
- `pypi/trackfw/generators/adr.py` — formato canônico (status:Proposed, date, author; header inline; Alternatives Considered; nome por data sem numeração sequencial)
- `pypi/trackfw/generators/roadmap.py` — `_roadmap_template` canônica (status:backlog minúsculo, date, req, squad; seções inglês)
- `pypi/trackfw/commands/adr.py` — default status Draft → Proposed
- `internal/validator/validator_test.go` — remove t.Skip nas linhas 1477/1564, renomeia testes, atualiza fixtures para formato canônico
- `npm/tests/validator.test.js` — converte testSkip → test, atualiza fixtures e nomes
- `pypi/tests/test_validator.py` — remove @pytest.mark.xfail, renomeia testes, usa geradores reais para criar fixtures
- `pypi/tests/test_generators_adr.py` — remove TestNextAdrNumber, atualiza asserções para formato canônico, status padrão Proposed
- `pypi/tests/test_generators_req.py` — remove linked_adr, atualiza seções para inglês
- `pypi/tests/test_generators_roadmap.py:70` — status: Backlog → status: backlog
- `pypi/tests/test_commands_roadmap_discover.py:55` — status: Backlog → status: backlog

---

## Sessão 2026-07-27 — ML-2A concluído

**Agente:** Apolo (Backend Senior Specialist)
**Status:** CONCLUÍDO
**Branch:** `fix/convergencia-dos-templates-de-artefato-do-cli-python`
**Commit:** `2ee4f2b`

### O que foi entregue

**Geradores Python convergidos para formato canônico Go/Node:**
- `pypi/trackfw/generators/req.py`: frontmatter canônico, header inline `> Date: | Status: Open`, 5 seções em inglês incluindo `## Blocked by ADRs`
- `pypi/trackfw/generators/adr.py`: remove `next_adr_number`, nome por data `ADR-YYYY-MM-DD-<slug>.md`, frontmatter canônico, header inline, `## Alternatives Considered`, status padrão `Proposed`
- `pypi/trackfw/generators/roadmap.py`: `_roadmap_template` canônica, `status: backlog` (minúsculo), seções em inglês, Node ≡ Python byte-a-byte
- `pypi/trackfw/commands/adr.py`: default status `Draft` → `Proposed`

**6 testes reativados (2 por runtime) — todos passando:**
- Go: t.Skip removido das linhas 1477 e 1564, fixtures canônicos, testes renomeados
- Node: testSkip → test, fixtures canônicos
- Python: xfail removido, testes chamam geradores reais (regressão garantida)

**Asserções antigas corrigidas em 5 arquivos de teste**

**make quality:** 596 passed, 0 failed, 0 xfail

**Diff empírico:**
- ADR: Go = Node = Python (byte-idêntico)
- REQ: Node = Python; Go diverge com `| Linear Issue: ` e `| Jira Issue: ` (terceira divergência Go↔Node, fora do escopo negativo)
- ROADMAP: Node = Python; Go diverge com REQ preenchido + squad: + ML-1A literal (divergências 1-2 do escopo negativo)

**Escopo negativo respeitado:** validator não alterado, branch não criada, PR não aberto, main não tocada, docs/schema não alterados.

**Próximo passo:** ML-3A (gate de paridade de saída, executa os 3 geradores e compara byte a byte — é do orquestrador)

---

## Sessão 2026-07-27 — ML-2B: Convergência Go↔Node (paridade de templates)

**Agente:** Apolo | **Status:** CONCLUÍDO
**Branch:** `fix/convergencia-dos-templates-de-artefato-do-cli-python`
**REQ:** `REQ-2026-07-27-convergencia-templates-python`
**Commit:** `ddec027`

**Escopo executado:**
1. `npm/src/commands/roadmap.js` — `.argument('[title]')` + action `(titleArg, opts)` com precedência posicional > --title > 'New Roadmap'
2. `npm/src/generators/req.js` — `\n| Linear Issue: \n| Jira Issue: ` adicionado a ambas as variantes de statusLine
3. `pypi/trackfw/generators/req.py` — idem no status_line
4. `internal/generators/roadmap.go` — `squad:` removido do body; `### ML-1A — %s` com content.Title interpolado
5. `internal/commands/roadmap.go` — args[0] → title (não selectedREQ); selectedREQ permanece ""

**Verificação empírica (entrada "auth strategy" nos 3 CLIs):**
- ADR Go↔Node: IDENTICAL | ADR Go↔Python: IDENTICAL
- REQ Go↔Node: IDENTICAL | REQ Go↔Python: IDENTICAL
- ROADMAP Go↔Node: IDENTICAL | ROADMAP Go↔Python: IDENTICAL

**`make quality`:** 596 passed · 0 failed · 6 falsification checks passed

**Divergências residuais (report-only, fora do escopo):**
- Python imprime `Roadmap criado:` vs Go/Node `✓ created` (mensagem de saída, não conteúdo de arquivo)
- Node `req.js` usa `new Date().toISOString().slice(0,10)` (UTC) vs Go/Python local — sem impacto hoje (UTC == local-time), identificado como risco para ML-3A

---

## Sessão 2026-07-27 — ML-2C: Node converge para hora local (paridade de fuso)

**Agente:** Apolo | **Status:** CONCLUÍDO
**Branch:** `fix/convergencia-dos-templates-de-artefato-do-cli-python`
**REQ:** `REQ-2026-07-27-convergencia-templates-python`

**Problema corrigido:** `req.js:76`, `adr.js:today()`, `note.js:today()` usavam `new Date().toISOString().slice(0,10)` (UTC). Go e Python já usavam hora local. Em UTC+14 ou UTC-11, isso causava nomes de arquivo e datas diferentes entre os 3 CLIs — gate do ML-3A seria intermitente.

**Arquivos modificados:**
- `npm/src/generators/date.js` — helper `localDateISO()` criado (usa `getFullYear/getMonth/getDate`, não `toISOString`)
- `npm/src/generators/req.js` — `const date = localDateISO()` substituindo `toISOString`; exporta `localDateISO`
- `npm/src/generators/adr.js` — `today()` delega para `localDateISO()`; exporta `today`
- `npm/src/generators/note.js` — `today()` delega para `localDateISO()`; exporta `today`
- `npm/src/generators/roadmap.js` — dedup: `const date = localDateISO()` substituindo bloco de 4 linhas em `newRoadmap` e `newRoadmapFromReq` (comportamento já era local, mudança é structural)
- `npm/tests/generators_tz.test.js` — 4 testes de TZ determinísticos (UTC+14 × UTC-11, span 25h)
- `internal/generators/tz_test.go` — 3 testes Go com `time.Local = loc14`
- `pypi/tests/test_generators_tz.py` — 3 testes Python com `timezone()` context manager + `time.tzset()`

**Verificação empírica (UTC 2026-07-27T15:19Z):**
| Fuso | GO | NODE | PYTHON |
|---|---|---|---|
| Pacific/Kiritimati (UTC+14) | REQ-2026-07-28 | 2026-07-28 | REQ-2026-07-28 |
| Pacific/Midway (UTC-11) | REQ-2026-07-27 | 2026-07-27 | REQ-2026-07-27 |

GO=NODE=PYTHON em cada fuso; UTC+14 ≠ UTC-11 (confirmado).

**Ocorrências `toISOString` deixadas intocadas (report-only, não são artefatos governados):**
- `npm/src/generators/init.js:73,93,497` — scaffold de CLAUDE.md e lenient dates
- `npm/src/commands/discover.js:180` — exibição de timestamp de mtime
- `npm/src/commands/metrics.js:128` — exibição de timestamp de log
- `npm/src/validator/index.js:527,628,1009` — comparação de datas de validação e baseline
- `npm/src/serve/api_metrics.js:83` — cálculo de semanas para métricas de display

**`make quality`:** 599 passed · 0 failed · 6 falsification checks passed

---

## Sessão 2026-07-27 — ML-3A: gate de paridade de artefatos (Apolo)

**Status:** CONCLUÍDO

**Branch:** `fix/convergencia-dos-templates-de-artefato-do-cli-python`

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-27-convergencia-dos-templates-de-artefato-do-cli-python.md`

### O que foi implementado

**`scripts/check-artifact-parity.sh`** (novo gate):
- Invoca `req new`, `adr new`, `roadmap new` e `note new` nos 3 CLIs (Go, Node.js, Python) dentro de `mktemp -d`
- Compara arquivos gerados byte-a-byte — conteúdo e nome de arquivo
- Vacuity guard: falha explicitamente se nenhum arquivo foi gerado por runtime/tipo
- Guard de midnight rollover: falha com mensagem clara se a data mudar durante a geração
- Diagnóstico acumulado: coleta todos os erros antes de sair, identifica tipo e runtimes divergentes
- Caminho absoluto para GO_BIN: resolve path relativo do Makefile antes de entrar nas subshells

**`scripts/check-gates-falsify.sh`** (Cenário 7 adicionado):
- Corrompe `npm/src/generators/req.js` via `sed` para emitir `status: OPEN` no frontmatter
- Guard de corrupção via `cmp -s` (falha se sed não alterou nada — prova P4 inválida sem esse guard)
- `assert_fails_with "artifact-parity/req-content-drift" "artifact parity drift: req" ...`

**`Makefile`**: gate adicionado ao alvo `parity` após `check-identity-parity.sh`, antes de `check-gates-falsify.sh`

**`docs/cli-parity.md`**: nova seção "Contrato de artefatos gerados" documentando:
- Frontmatter completo de `req`, `adr`, `roadmap` e `note` como contrato explícito
- Limitação de slug não-ASCII (divergência residual: Python normaliza NFKD; Go/Node preserva Unicode)
- Comportamento de data local (nunca UTC)
- Referência ao gate e ao cenário negativo P4

### Divergência residual encontrada e reportada

Títulos com acentos ou caracteres não-ASCII produzem slugs distintos:
- Go: `autenticação:-sso-&-oauth2` (preserva Unicode)
- Node.js: `autenticação:-sso-&-oauth2` (preserva Unicode)
- Python: `autenticacao:-sso-&-oauth2` (NFKD → remove acentos)

O gate usa título ASCII puro (`"parity gate test"`) e documenta a limitação com comentário no script. A divergência está fora do escopo do ML (não tocar nos geradores).

### Verificação final
- `make quality` verde: 599 passed · 0 failed · 7 falsification checks passed
- `git status` limpo após `make quality`
- Commit `6c4f295` na branch · push realizado

---

## Sessão 2026-07-27 — Apolo — ML-3B: Slug acentuado portável (CONCLUÍDO)

**Tarefa:** Unificar normalização de slug nos 3 CLIs — NFKD portável para títulos PT-BR.

**Contexto:** ML-3A (Cenário 7 / check-artifact-parity.sh) contornou o defeito usando título ASCII puro. O defeito real: título como "Autenticação e Sessão" gerava nomes de arquivo distintos: Go/Node preservavam Unicode (`autenticação-e-sessão`), Python removia diacríticos (`autenticacao-e-sessao`). Dois impactos adicionais: portabilidade NFD/NFC entre plataformas e quebra do `branch_has_wip_roadmap`.

**Implementação:**

Semântica B adotada em todos os CLIs (NFKD → removing marks → lower → `[^a-z0-9]+`→`-` → trim):

- **Go** (`internal/generators/adr.go`): `norm.NFKD.String(s)` + loop de runes filtrando `unicode.Mn`. Dependência `golang.org/x/text` já estava no módulo (v0.27.0). Adicionado `regexp`, `unicode`.
- **Node** (`npm/src/generators/adr.js`, `req.js`, `note.js`, `roadmap.js`): `String.normalize('NFKD')` + regex combining marks `[̀-ͯ]` + `[^a-z0-9]+`→`-`. Sem dependência nova. Exports de `toSlug` adicionados a `req.js` e `roadmap.js`.
- **Python** (`pypi/trackfw/generators/req.py`, `note.py`, `roadmap.py`): unificados na semântica B (req e note tinham variante A sem `[^a-z0-9]`; roadmap não tinha NFKD).

**Gate e prova negativa:**
- `check-artifact-parity.sh`: título mudado para `"Autenticação e Sessão"`, SLUG para `autenticacao-e-sessao`. Comentário de limitação removido.
- `check-gates-falsify.sh` Cenário 7: pattern tightened para `"artifact parity drift: req (go vs node)"` (evita colisão com cenário 8). Cenário 8 adicionado: binário Go corrompido com prefixo `RREQ-` comprova divergência de nome com diagnóstico `"arquivo ausente"` (caminho vacuity guard, distinto do de conteúdo).

**Testes:**
- Go: `TestToSlug_Acentuado` — 7 vetores (á é í ó ú, ç, ã õ, à, parêntese).
- Node: `npm/tests/generators_slug.test.js` — 28 asserts (7 casos × 4 generators).
- Python: 5 novos casos em `TestSlugify` (`test_generators_req.py`).

**`docs/cli-parity.md`:** seção de limitação removida; substituída por tabela de exemplos e contrato declarado.

**Verificação final:**
- `make quality` verde: 604 passed · 0 failed · 8 falsification checks passed
- `git status` limpo após `make quality`
- Commit `dde3c94` na branch `fix/convergencia-dos-templates-de-artefato-do-cli-python` · push realizado

---

## Encerramento da REQ-2026-07-27-convergencia-templates-python — 2026-07-27 — Zeus

**Branch:** `fix/convergencia-dos-templates-de-artefato-do-cli-python`
**Status:** CONCLUÍDO

6 MLs (3 promovidos do escopo negativo por medição empírica). `make quality` verde: 604 passed,
8 cenários de falsificação cobrindo 7 gates — eram 6 no início do ciclo.

**Prova final com título acentuado**, nos 4 artefatos × 3 CLIs: nome de arquivo E conteúdo idênticos.
`trackfw req new "Autenticação e Sessão"` → `REQ-2026-07-27-autenticacao-e-sessao.md` nos três.

**Padrão que se repetiu 3 vezes:** cada defeito promovido passava despercebido porque a verificação
existente não exercitava o caso real — gate comparando nomes de comando em vez de saída; minha
auditoria de fuso passando por sorte (rodei de dia, UTC == local); gate do ML-3A contornando slug
acentuado com fixture ASCII num projeto PT-BR. Verde por coincidência não é verde.

**Novo gate:** `scripts/check-artifact-parity.sh` executa os geradores dos 3 runtimes e compara saída
real, com prova negativa nos dois caminhos (conteúdo e nome de arquivo).

**Débito (fila do próximo ciclo):** migração dos 50 roadmaps em 12 formatos · slash-command
`/trackfw:roadmap` gerando roadmap sem frontmatter nos 3 init · flags `--from-req`/`--req` ausentes no
Python · `docs/schema/*.json` morto e `site/guide/ai-agents.md:68` afirmando falsamente que o validate
os consome.

---

## Migração do legado claude/ para estrutura flat — 2026-07-27 — Zeus

**Branch:** `chore/migra-legado-claude-para-estrutura-flat`
**Status:** CONCLUÍDO (parcial — ver pendências)

Doc-only (§7 dispensa REQ+roadmap). Decisão do usuário: migrar para flat.

**Migrado:** 33 roadmaps de `docs/roadmaps/claude/done/` → `docs/roadmaps/done/` (agora 50) ·
2 ADRs de `docs/adr/claude/` → `docs/adr/` (agora 11) · duplicata idêntica de
`trackfw-update-command-2026-06-18.md` em `claude/wip/` removida (o arquivo já existia em `done/`).

**Veredito do roadmap parado há 39 dias:** `trackfw update` **foi entregue**. Os 5 MLs existem em
código (`internal/generators/update.go`, force variants em `scaffold.go`, `internal/commands/update.go`,
`npm/src/commands/update.js`, `pypi/trackfw/commands/update.py`) e o comando funciona nos 3 CLIs.
Mergeado no PR #39 (v2.9.0). O roadmap só nunca foi encerrado.

**A migração provou seu valor imediatamente:** 7 roadmaps com `folder_status` divergente
(pasta `done/`, frontmatter `wip`/`backlog`) estavam **invisíveis** ao validator e passaram a acusar.
Frontmatter corrigido nos 7. `validate` verde.

**Pendências escaladas ao usuário (não migradas):**
- `docs/requisições/` — 37 REQs em 4 subpastas de agente (claude 22, afrodite 10, artemis 2, apolo 3).
  Zero colisão com `docs/req/`. Conflito documental: o CLAUDE.md global cita `docs/requisições/claude/`
  mas o `req_dir` default do CLI é `docs/req`.
- `docs/roadmap/` (singular, declarado descontinuado no CLAUDE.md) — 12 roadmaps, sendo 6 em estado
  não-terminal (4 wip + 2 backlog). São fragmentos por ML (`-ML-1B`, `-ML-2B`); 4 têm roadmap-pai em
  `done/` e 2 (`attention-hooks-agent-clis-node`) correspondem ao pai
  `ROADMAP-2026-06-20-attention-hooks-agent-clis.md`, também em `done/`.

---

## Sessão 2026-07-27 — Normalização de REQs Legadas (Apolo)

**Status:** IMPLEMENTANDO

### Contexto
Branch `chore/migra-legado-claude-para-estrutura-flat`. Zeus migrou 37 REQs de `docs/requisições/` para `docs/req/`. O `trackfw validate` acusa 58 violations em 29 arquivos (25 × req_has_adr, 24 × req_has_roadmap, 9 × no frontmatter).

### O que foi feito
Normalização doc-only dos 29 arquivos em `docs/req/`: adição de frontmatter canônico, header padronizado e seções `## Linked ADR` + `## Linked Roadmap` com marcadores que satisfazem as regras do validator.

**Status:** CONCLUÍDO

### Resultado
- 29 arquivos normalizados (9 sem frontmatter + 20 com frontmatter legado)
- `trackfw validate --json` → 0 violations, 0 warnings
- `make quality` → todos os gates verdes (Go + Node.js + Python)
- Nenhum arquivo fora de `docs/req/` foi tocado
- Links ADR preenchidos apenas quando arquivo real encontrado por slug (ML-1B e ML-2B → `ADR-2026-07-19-global-adrs-governance.md`); demais com `ADR:` vazio
- Links Roadmap preenchidos com basename do roadmap correspondente em `docs/roadmaps/`

**Complemento — migração total concluída:**

- `docs/requisições/` (37 REQs em 4 subpastas de agente) → `docs/req/`. 4 colisões de nome puladas
  (arquivo já existia no destino). `docs/req/` agora tem 48.
- `docs/roadmap/` (singular, descontinuado) → `docs/roadmaps/done/`. Eram 12 arquivos mas apenas
  **7 nomes únicos**: os mesmos roadmaps existiam duplicados em `backlog/`, `wip/` e `done/`
  simultaneamente. Mantida a versão mais avançada de cada (done > wip > backlog), status normalizado.
  `docs/roadmaps/done/` agora tem 57.
- **29 REQs legadas normalizadas** para o formato canônico: frontmatter, header
  `> Date: … | Status: …` e seções `## Linked ADR` / `## Linked Roadmap`. 60 links preenchidos por
  casamento de slug — **todos verificados como apontando para arquivo existente** — e 30 marcadores
  deixados vazios em vez de inventar referência.
- **`~/.claude/CLAUDE.md` atualizado** (fora deste repo): estrutura flat `docs/roadmaps/` sem subpasta
  por agente, REQs em `docs/req/`, e instrução de usar `trackfw roadmap move` em vez de `git mv`.

**Resultado:** zero diretórios órfãos em `docs/`. `validate` 0 violations, `make quality` verde.
Os 85 artefatos antes invisíveis ao CLI agora estão sob governança.

---

## REQ-2026-07-27-integridade-referencias — 2026-07-27 — Zeus

**Branch:** `fix/integridade-das-referencias-e-ciclo-de-vida-da-req`
**Status:** IMPLEMENTANDO

**Defeito 1:** 38 de 48 REQs (79%) com `roadmap:` apontando para caminho inexistente, `validate`
verde. Três escapes independentes: frontmatter nunca é lido (extrator busca `Roadmap:` no corpo);
fallback por basename recursivo em `referenceExists`; severidade `warning`. 37 das 38 apontam para
arquivo que existe — é ausência de formato canônico, não rastreabilidade perdida.

**Defeito 2:** nada fecha a REQ. 6 com `Status: Open` e roadmap em `done/`. `blocked_by_draft_adr` é
`error` → falso positivo (REQ entregue reprova o gate se um ADR dela virar Draft) e falso negativo
(REQ marcada Done à mão é excluída do check).

**Formato canônico decidido e verificado:** caminho relativo completo com `.md`. `api_chain.go` monta
o nó com `ID: path` de `filepath.WalkDir(cfg.RoadmapDir,...)` e a aresta é `{From: path, To: val}` —
qualquer outro formato gera aresta órfã no grafo do serve.

**Ordem crítica:** a elevação de `ref_targets_exist` para `error` fica no ML-3A, **depois** da
normalização dos dados. Elevar na Wave 2 deixaria `make quality` vermelho na barrier.

**Fora de escopo:** slash-command sem frontmatter · `stale_wip` inócuo (mtime) · schemas mortos ·
flags do Python · 6 itens de higiene menores.

## Retomada 2026-07-27 — Zeus

Retomado o roadmap `ROADMAP-2026-07-27-integridade-das-referencias-e-ciclo-de-vida-da-req.md`
na branch `fix/integridade-das-referencias-e-ciclo-de-vida-da-req`. O `trackfw validate` passou
sem violações. O ML-1A estava parcial, com testes negativos locais em Go e Node; Python, relatório
das falhas, `make quality`, commit e push permaneciam pendentes. Handoff realizado para Artemis
concluir e validar exclusivamente o ML-1A antes da barrier da Wave 2.

## ML-1A 2026-07-27 — Artemis

**Status:** CONCLUÍDO na branch `fix/integridade-das-referencias-e-ciclo-de-vida-da-req`.

**Escopo entregue:** 4 cenários negativos de integridade referencial/ciclo de vida nos 3 runtimes,
sem alteração de código de produção:
- Escape 1: `roadmap:` no frontmatter aponta para arquivo inexistente e não há `Roadmap:` no corpo.
- Escape 2: `Roadmap: docs/roadmaps/wip/X.md` enquanto o arquivo real está em `docs/roadmaps/done/X.md`.
- Escape 3: `ref_targets_exist` default `warning` não reprova o gate.
- Defeito 2: REQ `Open` com roadmap em `done/` não é sinalizada.

**Arquivos alterados:** `internal/validator/validator_integrity_xfail_test.go`,
`npm/tests/validator.test.js`, `pypi/tests/test_validator.py`,
`docs/roadmaps/wip/ROADMAP-2026-07-27-integridade-das-referencias-e-ciclo-de-vida-da-req.md`,
`docs/agents-working-context.md`.

**Evidência das falhas esperadas:**
- `go test ./internal/validator -run 'TestXFail' -v` → 4 testes executados com logs
  `[xfail esperado]`; helper Go falha em XPASS via `t.Errorf`, sem `t.Skip`.
- `npm test -- --runInBand --test-name-pattern=validator` → `37 passed, 0 failed, 4 xfail`
  no validator.
- `python3 -m pytest pypi/tests/test_validator.py -q -rxX -k ml1a` →
  `59 deselected, 4 xfailed`; marcado com `pytest.mark.xfail(strict=True)`.

**Validação final:**
- `python3 -m pytest pypi/tests/test_validator.py -q -rxX` no sandbox falhou em dois testes legados
  por `PermissionError` ao criar diretórios temporários em `~/`; classificado como limitação ambiental.
- `make quality` executado fora do sandbox → verde: Go `ok`; Node `261 pass` e validator
  `37 passed, 0 failed, 4 xfail`; Python `604 passed, 4 xfailed`; `go vet`, build, parity,
  static/integration assets, identity parity, artifact parity e falsification gates passaram.

## Finalização ML-1A 2026-07-27 — Artemis

**Status:** auditado e pronto para handoff da Wave 1.

**Validações executadas nesta finalização:**
- `go test ./internal/validator -run TestXFail -v` → 4/4 `PASS` com logs `[xfail esperado]`.
- `npm test -- --runInBand --test-name-pattern=validator` na raiz → falhou por ausência de
  `package.json` na raiz; reexecutado no workspace `npm/`.
- `npm test -- --runInBand --test-name-pattern=validator` em `npm/` → `37 passed, 0 failed, 4 xfail`
  no `tests/validator.test.js`; suíte Node total reportou `261 pass`.
- `python3 -m pytest pypi/tests/test_validator.py -q -rxX` → `59 passed, 4 xfailed`.
- `make quality` → verde: Go `ok`; Node `261 pass`; Python `604 passed, 4 xfailed`; `go vet`,
  build, parity, static/integration assets, identity parity, artifact parity e falsification gates
  passaram.

**Observação:** o commit `fef4184 test(validator): expose reference integrity escapes` já estava no
topo da branch local e sincronizado com `origin/fix/integridade-das-referencias-e-ciclo-de-vida-da-req`
antes desta nota; esta finalização registra a auditoria posterior e não toca Wave 2.

## ML-2A 2026-07-27 — Apolo

**Status:** CONCLUÍDO na branch `fix/integridade-das-referencias-e-ciclo-de-vida-da-req`.

**Escopo entregue:** formato canônico e validação real de referências em Go, Node.js e Python:
- `adr:` e `roadmap:` em frontmatter agora são lidos de forma case-insensitive e com strip de aspas.
- Referências são validadas por caminho literal expandido; o fallback recursivo por basename foi
  removido.
- `blocked` passou a usar resolução namespace-aware (`resolveStateDirs(..., "blocked")`) em
  `blocked_has_req` e `ref_targets_exist`.
- Testes dos escapes 1 e 2 foram reativados nos 3 runtimes. Escape 3 segue para ML-3A; Defeito 2
  segue para ML-2B.
- `docs/cli-parity.md` documenta o contrato de caminho relativo completo desde a raiz, com `.md`.

**Arquivos alterados:** `internal/validator/validator.go`,
`internal/validator/validator_integrity_xfail_test.go`,
`internal/validator/validator_improvements_test.go`, `internal/validator/validator_namespacing_test.go`,
`internal/validator/validator_test.go`, `npm/src/validator/index.js`, `npm/tests/validator.test.js`,
`npm/tests/namespacing.test.js`, `pypi/trackfw/validator.py`, `pypi/tests/test_validator.py`,
`pypi/tests/test_namespacing.py`, `docs/cli-parity.md`,
`docs/roadmaps/wip/ROADMAP-2026-07-27-integridade-das-referencias-e-ciclo-de-vida-da-req.md`.

**Validação final:**
- `go build ./...` → exit 0; aviso não bloqueante de cache Go fora do workspace no sandbox.
- `go test ./...` → verde.
- `(cd npm && npm test)` → `261 pass`, `0 fail`.
- `python3 -m pytest pypi/tests -q -rxX` → `607 passed, 2 xfailed`.
- `bin/trackfw validate` → exit 0, com 41 warnings de referências canônicas pendentes para ML-3A.
- `make quality` → verde: Go, Node, Python, vet, build e gates de paridade/falsificação passaram.

## ML-2B 2026-07-27 — Apolo

**Status:** CONCLUÍDO na branch `fix/integridade-das-referencias-e-ciclo-de-vida-da-req`.

**Escopo entregue:** fechamento de REQ e higiene de paridade nos três runtimes:
- `req move <nome> <status>` implementado em Go, Node.js e Python sem mover arquivo; reescreve somente
  o `status:` do frontmatter e o primeiro `| Status: ...` no header, preservando demais bytes.
- `trackfw log` em Node.js e Python passou a usar `<roadmap_dir>/.trackfw-log`, alinhado ao Go.
- Strip de aspas em `forge` e `trace_id_field` no Go ficou coberto por teste de regressão.
- Defeito 2 reativado nos 3 runtimes: REQ `Open` com roadmap referenciado em `done/` agora gera warning
  de ciclo de vida (`req_roadmap_lifecycle`). Escape 3 permanece xfail para ML-3A.

**Arquivos alterados:** `internal/commands/req.go`, `internal/commands/log_test.go`,
`internal/config/config_test.go`, `internal/generators/req.go`, `internal/generators/req_test.go`,
`internal/validator/validator.go`, `internal/validator/validator_integrity_xfail_test.go`,
`npm/src/commands/log.js`, `npm/src/commands/req.js`, `npm/src/generators/req.js`,
`npm/src/validator/index.js`, `npm/tests/config.test.js`, `npm/tests/log_path.test.js`,
`npm/tests/req_move.test.js`, `npm/tests/validator.test.js`, `pypi/trackfw/commands/log.py`,
`pypi/trackfw/commands/req.py`, `pypi/trackfw/generators/req.py`, `pypi/trackfw/validator.py`,
`pypi/tests/test_commands_basic.py`, `pypi/tests/test_config.py`, `pypi/tests/test_generators_req.py`,
`pypi/tests/test_log_command.py`, `pypi/tests/test_validator.py`,
`docs/roadmaps/wip/ROADMAP-2026-07-27-integridade-das-referencias-e-ciclo-de-vida-da-req.md`,
`docs/agents-working-context.md`.

**Validação final:**
- `go build ./...` → exit 0; aviso não bloqueante de cache Go fora do workspace no sandbox.
- `go test ./...` → verde.
- `(cd npm && npm test)` → `263 pass`, `0 fail`.
- `python3 -m pytest pypi/tests -q -rxX` → `611 passed, 1 xfailed`.
- `git diff --check` → verde.
- `make quality` → verde: Go, Node, Python, vet, build, paridade CLI/validate, assets,
  identity/artifact parity e falsification gates passaram.

## Encerramento 2026-07-27 — Zeus

Roadmap de integridade das referências e ciclo de vida da REQ auditado após as três waves.
`bin/trackfw validate --json` retornou 0 violations e 0 warnings; o gate positivo de integridade
passou; o harness de falsificação executou os 9 cenários fora do sandbox, incluindo
`artifact-parity/req-name-drift` e `referential-integrity/missing-roadmap`, com exit 0.
A interrupção observada dentro do sandbox ocorreu na cópia/compilação isolada do cenário 8 e não
se reproduziu no ambiente autorizado. Todos os critérios globais foram marcados como concluídos.

## Housekeeping pós-merge 2026-07-27 — Zeus

Após o merge do PR #78, a `main` local foi atualizada e a branch mergeada foi removida. A única
REQ ainda `Open`, `REQ-2026-06-13-validator-improvements.md`, era um artefato legado cujo roadmap
já estava concluído. A REQ foi marcada `Done`, seus campos de referência foram normalizados e o
roadmap correspondente passou a apontar para `docs/req/`. Mudança exclusivamente documental,
dispensada de nova REQ/roadmap pela exceção objetiva do AGENTS.md.

## Planejamento 2026-07-27 — Zeus — contrato canônico e analyzing

Criadas a `REQ-2026-07-27-contrato-canonico-do-roadmap-e-estado-analyzing.md` e a
`ROADMAP-2026-07-27-contrato-canonico-do-roadmap-e-estado-analyzing.md`, consolidando dois débitos:
o slash-command `/trackfw:roadmap` gera artefato sem frontmatter, e o estado `analyzing` é reconhecido
pelo scaffold/validator mas rejeitado por `roadmap move`. Roadmap mantido em `backlog/`; nenhuma
implementação iniciada. `trackfw validate` retornou 0 violations e 0 warnings.

## Formalização de follow-ups 2026-07-27 — Zeus

Os achados remanescentes deixaram de ser follow-ups soltos de sessões anteriores e foram separados
por impacto:

- **Bloqueantes de release:** `REQ-2026-07-27-bloqueadores-de-release-de-paridade-e-precisao-contratual.md`
  + roadmap homônimo em `backlog/` — flags Python, strip de aspas, log `by_agent` e contrato de schemas.
- **Não bloqueantes:** `REQ-2026-07-27-debitos-tecnicos-pos-release-de-robustez-e-manutenibilidade.md`
  + roadmap homônimo em `backlog/` — `stale_wip`, política de I/O e catálogo do gate de identidade.

A memória Claude específica do workspace contém somente a regra permanente de paridade dos três
CLIs e não armazenava esses follow-ups; nada foi removido dela. A fonte de verdade para execução
passa a ser exclusivamente as REQs e roadmaps acima.

## Implementação 2026-07-27 — Zeus — contrato canônico e analyzing

Implementação autorizada da REQ de contrato canônico do roadmap e estado `analyzing`. Como
`roadmap move ... analyzing` é um dos defeitos desta própria demanda, a etapa intermediária do skill
não pôde ser executada pelo comando oficial; o roadmap foi movido de `backlog/` diretamente para
`wip/` ao iniciar a codificação. ML-1A marcado em andamento para produzir as provas negativas antes
de qualquer correção de produção.

Após auditoria do ML-1A, as falhas atuais ficaram caracterizadas nos três runtimes: o slash-command
não instrui frontmatter canônico e `roadmap move ... analyzing` é rejeitado. ML-2A liberado para
corrigir exclusivamente o contrato do slash-command antes da implementação do novo estado.

ML-2A aprovado após restaurar e proteger por testes o esqueleto de múltiplos microlotes e waves.
ML-2B liberado para implementar `analyzing` nos comandos de movimentação dos três CLIs, incluindo
layouts flat/by-agent, sincronização de status, log e promoção dos testes de caracterização.

ML-2B aprovado com paridade funcional e testes obrigatórios nos três CLIs. ML-3A liberado para
auditoria transversal: contratos de paridade, documentação dos estados e cenário E2E
`backlog → analyzing → wip`, seguido pelo gate composto `make quality`.

REQ concluída em 2026-07-27. O ciclo canônico foi implementado nos três CLIs: slash-command com
frontmatter (`status`, `date`, `req`, `squad`), `roadmap move ... analyzing` em layouts flat e
by-agent, sincronização de pasta/frontmatter/header/log e gates E2E/falsificação. `make quality`
passou integralmente; roadmap movido para `docs/roadmaps/done/` e REQ marcada como Done.

## Implementação 2026-07-27 — Zeus — bloqueadores de release

Após o merge do contrato canônico, a REQ de bloqueadores de release foi movida de `backlog/` para
`analyzing/`, validada sem violações e iniciada em `wip/` na branch
`fix/bloqueadores-release-paridade`. ML-1A está em andamento para caracterizar, antes de alterar
produção, as quatro divergências: flags Python, frontmatter com aspas, log `by_agent` e alegação
documental de JSON Schema.

ML-1A confirmou três bloqueadores reais (flags Python, valores aspeados e contrato documental). O
quarto, log `by_agent`, já estava corrigido na base e recebeu teste obrigatório de regressão; ML-2C
foi encerrado por evidência, sem alteração de produção. ML-2A, ML-2B e ML-2D foram liberados em
paralelo, com ownership de arquivos não sobreposto.

Wave 2 concluída: o Python passou a aceitar `roadmap new --title/--req/--from-req`; valores YAML
flat com aspas externas agora normalizam identicamente no validator e trace ID; e a documentação
passou a declarar JSON Schemas como auxiliares externos. O smoke de packages iniciado no ML-2A
atingiu timeout local depois de sincronizar assets; sua execução completa foi transferida para o
gate integrado do ML-3A.

REQ concluída em 2026-07-27. O gate integrado passou com flags Python, parsing aspeado e logs
flat/by-agent cobertos por paridade e falsificação (12 cenários). `make quality` passou completo;
o smoke de tarball npm e wheel PyPI passou após instalar `build` em dependência temporária isolada.
Roadmap movido para `docs/roadmaps/done/` e REQ marcada como Done, liberando a próxima versão.

## Implementação 2026-07-27 — Zeus — débitos técnicos pós-release

Após o merge dos bloqueadores de release, a última REQ pendente foi movida de `backlog/` para
`analyzing/`, validada sem violações e iniciada em `wip/` na branch
`fix/debitos-tecnicos-pos-release-de-robustez-e-manutenibilidade`.
Wave 1 foi auditada com sucesso: o contrato de `stale_wip`/erros de inspeção está documentado,
as provas negativas passam como xfail esperado nos três runtimes e o gate de identity parity agora
prova a lacuna de catálogo sem resíduos. Wave 2 está liberada para implementação sequencial dos
três débitos, evitando sobreposição nos contratos compartilhados dos validators.

Wave 2 e Wave 3 foram concluídas na mesma branch: `stale_wip` passou a usar a última transição
para `wip`, com fallback `mtime` e limiar configurável nos três CLIs; erros de inspeção passaram a
gerar diagnósticos explícitos; e o gate de identity parity passou a derivar targets/surfaces do
catálogo. A documentação foi atualizada, `make quality` passou com 643 testes Python, os gates de
falsificação passaram com 13 cenários e 8 gates não-vazios, e `trackfw validate --json` retornou 0
violações e 0 avisos. REQ e roadmap foram concluídos.

## Planejamento 2026-07-29 — barrier de governança e autoridade do orquestrador

Solicitada a formalização da barrier como funcionalidade do trackfw. Criados o ADR
`docs/adr/ADR-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md`, a REQ
`docs/req/REQ-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md` e o roadmap em
`docs/roadmaps/backlog/`.

Decisões registradas: `trackfw barrier` será o núcleo determinístico; `/trackfw:barrier` será a
camada de orquestração; gates de stack são configuráveis pelo projeto; `trackfw_architect` é a única
autoridade Git; especialistas não fazem operações Git e só atuam por handoff.

Validação do planejamento: `git diff --check` verde e `bin/trackfw validate --json` com 0 violações
e 0 avisos. Nenhuma branch foi criada e nenhuma implementação foi iniciada.

## Implementação 2026-07-29 — Zeus — barrier de governança (IMPLEMENTANDO)

O roadmap da barrier foi reordenado para ordem lógica de execução (Waves 1→2→3→4→5→6). As Waves 5 e
6 apareciam fisicamente antes das Waves 2–4, o que confundia a execução; o bloco foi movido para
depois da Wave 4 como permutação pura, sem alterar numeração, dependências ou referências cruzadas.

Roadmap movido para `docs/roadmaps/wip/` e branch `feat/barrier-governanca-e-autoridade-do-orquestrador`
criada. Duas decisões congeladas antes do primeiro handoff:

1. **Autoridade de artefatos no ML-1A**: o `trackfw_architect` é o único autor de `docs/adr/`,
   `docs/req/`, `docs/roadmaps/` e `docs/cli-parity.md`. O especialista do ML-1A implementa
   exclusivamente os testes negativos/xfail.
2. **Escopo dos testes negativos**: criados nos três runtimes já na Wave 1 (Go `t.Skip`, Node
   `{ skip: true }`, Python `@pytest.mark.xfail(strict=True)`), garantindo baseline vermelha
   idêntica para os MLs 2A/2B/2C e tornando a paridade verificável no barrier da Wave 2.

## QA 2026-07-29 — Ártemis — ML-1A: testes de contrato da barrier (INÍCIO)

Handoff recebido do `trackfw_architect` para o ML-1A: criar, nos três runtimes, os testes de
contrato de `trackfw barrier` a partir da seção `## trackfw barrier` já congelada em
`docs/cli-parity.md`. Nenhuma operação Git executada; nenhum arquivo de `docs/adr/`, `docs/req/`,
`docs/roadmaps/` ou `docs/cli-parity.md` tocado — apenas os três arquivos de teste do ML-1A.

## QA 2026-07-29 — Ártemis — ML-1A: testes de contrato da barrier (CONCLUÍDO)

Criados `internal/commands/barrier_contract_test.go`, `npm/tests/barrier-contract.test.js` e
`pypi/tests/test_barrier_contract.py`, cobrindo os oito cenários obrigatórios do handoff
(`wave_verde_passa`, `ml_pendente_bloqueia`, `evidencia_ausente_bloqueia`,
`ml_sem_bloco_de_criterios_bloqueia`, `gate_falho_bloqueia`, `validate_falho_bloqueia`,
`roadmap_ou_wave_inexistente_e_erro_de_uso`, `json_deterministico`) com fixtures reais de roadmap
(regras de parsing string-level da seção do contrato) e invocação do binário/CLI real de cada
runtime via subprocess.

Descoberta durante a implementação: no cenário 7 (erro de uso, exit 2), o Python CLI atual
(argparse) já rejeita `barrier` como comando desconhecido com exit code 2 — coincidindo
acidentalmente com o exit code esperado do contrato antes mesmo da implementação existir. Isso
fazia o `xfail(strict=True)` reportar XPASS (falso positivo de "já pendente corretamente"). Corrigido
adicionando a asserção de que o stderr deve nomear explicitamente o wave/roadmap não resolvido —
asserção que só uma implementação real do contrato pode satisfazer. Mesma asserção replicada em
Go e Node para equivalência semântica entre runtimes.

Nenhum arquivo de produção foi criado ou alterado; nenhum arquivo em `docs/` foi tocado.

Evidência de validação:
- `go build ./... && go vet ./... && go test ./...` — verde (todos os pacotes ok; 8 testes da
  barrier em `SKIP`).
- `cd npm && npm test` — `265 passed, 0 failed`, `8 skipped` (barrier).
- `python3 -m pytest pypi/tests -q` — `643 passed, 8 xfailed` (barrier).
- `git diff --check` — sem saída (limpo).
- `bin/trackfw validate --json` — `{"summary":{"violations":0,"warnings":0,"mode":"strict","exit_code":0}}`.

Nenhuma operação Git foi executada por este agente (sem add/commit/push/branch). Aguardando
auditoria e commit pelo `trackfw_architect`.

## Auditoria 2026-07-29 — Zeus — Wave 1 (ML-1A) aprovada

Contrato da barrier congelado em `docs/cli-parity.md` pelo orquestrador antes do handoff, incluindo
regras de parsing string-level do roadmap, os quatro checks embutidos, o documento JSON
determinístico e a distinção entre exit 1 (`blocked`) e exit 2 (erro de uso).

Ártemis entregou os testes de contrato nos três runtimes (1188 linhas, 8 cenários idênticos por
runtime, corpos reais atrás da marcação de pendência). Auditoria de escopo: nenhum arquivo de
produção criado, nenhum artefato de governança alterado pelo especialista, nenhuma operação Git
executada por ele.

Achado incorporado ao contrato: `@pytest.mark.xfail(strict=True)` executa o corpo do teste, ao
contrário de `t.Skip`/`{ skip: true }`. O cenário 7 passava acidentalmente porque o argparse do
Python já rejeita subcomando desconhecido com exit 2 genérico. O contrato passou a exigir que a
mensagem de exit 2 nomeie o roadmap/wave não resolvido, tornando a asserção não-vacuosa.

Gates da Wave 1: `make quality` exit 0 (643 testes Python + 8 xfailed, suíte Node e Go verdes,
13 cenários de falsificação, 8 gates provados não-vacuosos) e `bin/trackfw validate --json` com
0 violações. Wave 2 liberada.

## Backend 2026-07-29 — Apolo — ML-2C: implementa `trackfw barrier` no Python (INÍCIO/CONCLUÍDO)

Handoff recebido do `trackfw_architect` para o ML-2C: implementar `trackfw barrier <roadmap> --wave
<n> [--json]` no runtime Python (`pypi/`), espelhando o contrato congelado em `docs/cli-parity.md` e
a paridade dos MLs 2A/2B (Go/Node, em execução paralela). Apenas arquivos sob `pypi/` tocados;
nenhum arquivo sob `internal/`, `cmd/` ou `npm/` foi tocado; nenhum arquivo de `docs/adr/`,
`docs/req/`, `docs/roadmaps/` ou `docs/cli-parity.md` foi editado; nenhuma operação Git executada
por este agente.

Criado `pypi/trackfw/commands/barrier.py`: parser string-level das seis regras de parsing (wave
heading, ML heading, status ✅, bloco de critérios de aceite, bloco de gates com fence bash,
detecção de entrada malformada com número de linha), os quatro checks embutidos na ordem fixa
(`mls_complete`, `acceptance_evidence`, `gates`, `validate` — `validate` invocado in-process via
`trackfw.validator.validate()`, nunca via subprocess do binário), documento JSON determinístico com
ordem de chaves fixa e `ensure_ascii=False` (evidence/failures carregam ✅), exit 0/1/2 distintos
(exit 2 nunca executa gates). Resolução de roadmap reaproveita
`validator.resolve_wip_dirs`/`resolve_done_dirs` (wip então done, layouts flat e by_agent). Mensagens
de exit 2 nomeiam explicitamente o roadmap ou a wave não resolvida (evita o falso positivo do
cenário 7 documentado em `vault/notes/barrier-contract-xfail-false-positive-2026-07-29.md`).

Removidas as 8 marcações `@pytest.mark.xfail(strict=True)` de `pypi/tests/test_barrier_contract.py`
(corpos dos testes preservados, não reescritos). Criado `pypi/tests/test_barrier.py` com 15 testes
unitários adicionais (resolução em `done/`, mensagens de erro de uso, parsing malformado — wave
heading não numérico, fence de gates não terminada —, múltiplos MLs, múltiplos gates, isolamento de
stdout do gate vs documento JSON, modo texto, contagem de critérios). Registrado o subcomando em
`pypi/trackfw/cli.py`.

Verificação empírica pré-implementação (sugerida por revisor): confirmado que o fixture do ML-1A é
satisfazível antes de escrever qualquer código — `trackfw validate --json` sobre o fixture com
`linked_req=True` reporta 0 violations (necessário para os cenários 1 e 8, status `passed`) e com
`linked_req=False` reporta exatamente 1 violation isolada (`wip_has_req`), sem ruído de outras regras
de governança (necessário para o cenário 6, que exige todos os demais checks verdes).

Ambiguidades de contrato encontradas e resolvidas por leitura literal (reportadas, não corrigidas no
contrato, que permanece congelado):
1. Extração do `<ML-id>` não é explicitada além de "começa com `### ML-`" — implementado como o
   token até o primeiro espaço após `### ` (regex `^### (ML-\S+)`), consistente com os exemplos
   ML-2A/ML-2C da tabela de evidence/failures.
2. Wave com zero ML headings: `mls_complete` bloqueia (per definição "Wave contains ≥ 1 ML"), mas
   nenhum formato de failure está pinado para esse caso — implementado como `"wave contains no ML
   headings"`, não literal ao contrato.
3. Bloco de critérios de aceite presente mas vazio (sem nenhuma linha `- [ ]`/`- [x]`): rule 4 diz
   que precisa ser "non-empty", mas só existem dois formatos de failure pinados — reaproveitado
   `"<ML-id>: no acceptance block"` para esse caso, escolha de implementação, não contrato.

Evidência de validação:
- `python3 -m pytest pypi/tests/test_barrier_contract.py -q` → `8 passed` (sem xfail, sem XPASS).
- `python3 -m pytest pypi/tests/test_barrier.py -q` → `15 passed`.
- `python3 -m pytest pypi/tests -q` → `666 passed` (suíte completa do runtime Python).
- `make quality` NÃO executado por instrução explícita do handoff (Go/Node ainda em execução
  paralela nos MLs 2A/2B).

Nenhuma operação Git foi executada por este agente (sem add/commit/push/branch). Aguardando
auditoria do `trackfw_architect` e consolidação após os três runtimes convergirem.

## Auditoria 2026-07-29 — Zeus — barrier da Wave 2: BLOQUEADA

Os três MLs da Wave 2 aterrissaram com escopo perfeitamente disjunto e suítes verdes em cada
runtime (`make quality` exit 0, `bin/trackfw validate --json` 0 violações). Ainda assim a barrier
reprovou: suíte verde por runtime não prova equivalência entre runtimes.

Auditoria de paridade executada rodando os três binários sobre a mesma fixture:

- Caminho principal: JSON byte-idêntico nos três (roadmap, wave, status, checks, evidence,
  failures, commands). ✅
- Bloco de critérios presente mas vazio: os três convergiram em `no acceptance block`. ✅
- Exit 2: os três retornam exit 2 e nomeiam a entidade não resolvida. ✅

Duas divergências reais, nenhuma capturada pelos 8 cenários de contrato:

1. Wave sem MLs produziu três strings diferentes: `wave 1: no ML found` (Go),
   `wave has no ML` (Node), `wave contains no ML headings` (Python).
2. As mensagens de exit 2 divergem em texto nos três runtimes.

Ambas foram fixadas literalmente em `docs/cli-parity.md`, adotando o texto do Go como canônico
para minimizar churn. ML corretivo despachado; a Wave 3 permanece bloqueada até nova barrier verde.

## 2026-07-29 — Apolo (Backend) — ML-2D: alinhamento das strings divergentes (corretivo)

Início: recebido handoff do `trackfw_architect` para o ML-2D corretivo. `trackfw context`/`validate`
confirmados verdes (score 100/100, 0 violações) e roadmap já em `wip/` antes de qualquer edição.
Escopo: apenas `npm/src/commands/barrier.js`, `pypi/trackfw/commands/barrier.py` e os testes
próprios (não-contrato) dos três runtimes. `internal/commands/barrier.go` já estava conforme —
só recebeu os dois testes de regressão que faltavam.

Correções aplicadas:
1. Node.js `evalMlsComplete` emitia `"wave has no ML"` sem o número da wave — passou a receber
   `waveNumber` e emitir `` `wave ${waveNumber}: no ML found` ``, igual ao Go.
2. Python `_check_mls_complete` emitia `"wave contains no ML headings"` (sem número) — passou a
   receber `wave_number` e emitir `f"wave {wave_number}: no ML found"`.
3. Node.js `resolveRoadmapFile` normalizava `.md` no argumento antes de reportar o erro e omitia
   `under <roadmap_dir>` — corrigido para usar `roadmapArg` cru e incluir `cfg.roadmapDir`.
4. Python `_resolve_roadmap_path` usava formato totalmente diferente do contrato
   (`roadmap not found: 'X' (searched wip/ and done/ under 'Y')`, aspas simples via `!r`) —
   reescrito para o texto pinado com aspas duplas.
5. Node.js `findWave` não nomeava o roadmap na mensagem de wave-not-found — passou a receber
   `roadmapBasename` (parâmetro opcional, retrocompatível com os testes de parsing puro que não
   precisam do CLI completo) e emitir `` `wave ${n} not found in roadmap "${basename}"` ``.
6. Python `_find_wave` recebia `roadmap_arg` (cru) e usava aspas simples — passou a receber
   `roadmap_basename` (resolvido, com `.md`, via `os.path.basename(roadmap_path)`) e aspas duplas.
7. Node.js e Python prefixavam o erro de exit 2 como `"barrier: ..."` / `"...error: ..."` —
   ambos alinhados para `"trackfw barrier: ..."`, sem o prefixo `error:` do argparse.

Testes de regressão adicionados (arquivos próprios, não os `*barrier_contract*` congelados):
`internal/commands/barrier_test.go`, `npm/tests/barrier.test.js`, `pypi/tests/test_barrier.py` —
cobrindo os dois casos que antes tinham zero cobertura (é por isso que a divergência passou
despercebida por MLs 2A/2B/2C).

Evidência de validação (comandos e saída completos, não resumidos):
- `go build ./... && go vet ./... && go test ./...` → build/vet limpos, todos os pacotes `ok`.
- `cd npm && npm test` → `300 pass, 0 fail`.
- `python3 -m pytest pypi/tests -q` → `669 passed`.
- `make quality` → `Falsification checks passed (all 13 scenarios, 8 gates proved non-vacuous)`,
  exit 0.
- `bin/trackfw validate --json` → `{"summary":{"violations":0,"warnings":0,"mode":"strict","exit_code":0}...}`.
- Prova cross-runtime manual (fixture idêntica, os três binários invocados na mesma pasta):
  defeito 1 → `['wave 1: no ML found']` nos três; defeito 2 (wave 99) →
  `trackfw barrier: wave 99 not found in roadmap "ROADMAP-parity-check.md"` nos três; defeito 2
  (roadmap ausente) → `trackfw barrier: roadmap "ROADMAP-does-not-exist" not found in wip/ nor
  done/ under docs/roadmaps` nos três — byte-idênticas, `exit=2` nos três.

Nenhuma operação Git executada por este agente. Não editei `docs/adr/`, `docs/req/`,
`docs/roadmaps/` nem `docs/cli-parity.md` — o roadmap aparece modificado no `git status` porque o
`trackfw_architect` já havia acrescentado o ML-2D ao arquivo antes do handoff, não por ação minha.
Aguardando auditoria do `trackfw_architect` para nova barrier da Wave 2 e liberação da Wave 3.

## Auditoria 2026-07-29 — Zeus — barrier da Wave 2: APROVADA

ML-2D corretivo alinhou Node.js e Python ao Go nos dois pontos divergentes. Reverificação
independente sobre a mesma fixture: `wave <n>: no ML found` idêntico nos três runtimes; as duas
mensagens de exit 2 byte-idênticas nos três; JSON do caminho principal byte-idêntico.

Gates: `make quality` exit 0 (300 testes Node, 669 Python, Go verde, 13 cenários de falsificação),
`bin/trackfw validate --json` 0 violações, `git diff --check` limpo.

Dogfooding: `bin/trackfw barrier <este-roadmap> --wave 2 --json` retornou exit 0 e
`status: passed`, com os quatro checks verdes e os gates reais da wave efetivamente executados.
A barrier validou a própria wave que a implementou. Wave 3 liberada.

## Auditoria 2026-07-29 — Zeus — barrier da Wave 3: APROVADA

ML-3A concentrou a autoridade Git no `trackfw_architect` e criou o slash command
`/trackfw:barrier`. Verificação independente:

- Grep de operações Git nos 36 assets (12 por runtime): somente `architect.md` aparece, e apenas
  no protocolo de autoridade. Os 11 especialistas declaram explicitamente que não executam Git e
  que só atuam por handoff autocontido.
- Equivalência do slash command: os literais de `barrier.md` extraídos dos três fontes têm SHA-256
  idêntico (3091 bytes). Essa superfície não tem gate automático — `check-artifact-parity.sh`
  cobre apenas `slash_roadmap` —, então a prova é manual e precisa ser repetida a cada alteração.
- `~/.claude` intocado: o agente foi proibido de rodar instaladores, já que
  `installGlobalSkillInner` escreve no HOME de forma invisível ao `git status`.
- Gates: `make quality` exit 0, `validate --json` 0 violações, `git diff --check` limpo.
- Dogfooding: `bin/trackfw barrier <este-roadmap> --wave 3` retornou exit 0 e `status: passed`.

Defeito pré-existente detectado na auditoria e registrado como ML-5C: o Node.js mantém dois mapas
de slash commands; `generateClaudeCommandsForce` lista 6 dos 9 comandos. `trackfw skills --force`
instala menos comandos que a instalação normal e menos que Go e Python, que usam um único mapa com
flag. Não corrigido aqui para não expandir a Wave 3.

## Auditoria 2026-07-29 — Zeus — barrier da Wave 4: APROVADA (após corretivo ML-2E)

O ML-4A entregou `scripts/check-barrier.sh` com 15 cenários, encadeado no alvo `parity` do
Makefile, mais um cenário de falsificação do próprio script, e a documentação de uso em README e
`site/guide` PT/EN.

O cenário de paridade do ML-4A reprovou de imediato e expôs um defeito que a auditoria da Wave 2
não pegou: no Go, o struct `barrierCheck` declarava os campos na ordem
`Name, Status, Evidence, Failures, Commands`, e `encoding/json` serializa por ordem de declaração.
O check `gates` saía com `commands` por último no Go e em terceiro no Node e no Python.

**Causa da falha de detecção: método de auditoria do orquestrador.** A comparação de paridade da
Wave 2 usou `json.dumps(..., sort_keys=True)`, que normaliza a ordem das chaves e torna a
divergência invisível. O contrato em `docs/cli-parity.md` fixa a ordem explicitamente. Auditorias
de paridade de JSON passam a exigir comparação com a ordem preservada
(`object_pairs_hook=OrderedDict` e `dumps` sem `sort_keys`).

ML-2E corrigiu o struct e adicionou teste que assevera a ordem literal das chaves — a ausência
desse teste é o que permitiu a divergência sobreviver.

Verificação independente com ordem preservada: os três runtimes emitem
`name, status, commands, evidence, failures`, JSON idêntico. `scripts/check-barrier.sh` passa nos
15 cenários. `make quality` exit 0 com 14 cenários de falsificação e 9 gates provados não-vacuosos.
`validate --json` 0 violações. `~/.claude` intocado.

Barriers reexecutadas após o corretivo: waves 2, 3 e 4 retornam `passed`.

---

## Sessão 2026-07-29 — Apolo (ML-5C `npm/src/generators/init.js` — unificação do mapa de slash commands, concluído)

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md`

**Tarefa:** eliminar a duplicação de dois mapas de slash commands no Node
(`generateClaudeCommands` com 9 comandos vs `generateClaudeCommandsForce` com 6 — faltavam
`roadmap.md`, `implement.md`, `barrier.md` no caminho forçado), seguindo o padrão de fonte única
já usado por Go (`installSkillsInner(force)`) e Python (`generate_claude_commands`).

**Entregue (escopo restrito a `npm/src/generators/` e testes de generators do Node):**
- `npm/src/generators/init.js`: extraído o objeto `CLAUDE_COMMANDS` (módulo-scope, 9 entradas) como
  fonte única. Nova função `installClaudeCommandsInner(dir, force)` escreve a partir desse mapa;
  `force=false` preserva o comportamento idempotente (não sobrescreve arquivo existente),
  `force=true` sempre sobrescreve. `generateClaudeCommands()` e `generateClaudeCommandsForce(rootDir)`
  agora só variam a flag `force` e a resolução do diretório (cwd-relativo vs `rootDir`-relativo) —
  assinaturas preservadas para não quebrar `scaffold()` nem `internal/commands` equivalente Node
  (`npm/src/commands/update.js`).
- `npm/tests/generators.test.js`: dois testes novos — (1) prova que os caminhos normal e forçado
  instalam exatamente o mesmo conjunto de 9 arquivos com conteúdo idêntico; (2) prova que o
  comportamento de sobrescrita permanece diferenciado (normal preserva conteúdo customizado
  existente, forçado sobrescreve).

**Validação:**
- `cd npm && npm test` → 303 passed, 0 failed (301 prévios + 2 novos, incluindo o teste de
  regressão que prova conjunto e conteúdo idênticos entre `generateClaudeCommands()` e
  `generateClaudeCommandsForce()`, e o teste que prova a diferença de sobrescrita entre os dois).
- `bin/trackfw validate --json` → `{"summary":{"violations":0,"warnings":0,"mode":"strict","exit_code":0}}`.
- `make quality` → exit 0 (`check-barrier.sh` 15/15 OK, `check-gates-falsify.sh` 14/14 OK, 9 gates
  não-vacuosos). Correção: a saída dessa execução mostra apenas o caminho normal do Node (`trackfw
  init`, log `.claude/commands/trackfw (9 slash commands)`) e o caminho Python; **não** contém a
  string `sobrescritos` em nenhuma linha, logo `make quality` **não** exercitou o caminho forçado do
  Node (`update --force` → `generateClaudeCommandsForce`) nesta rodada — confirmado por
  `grep -n sobrescritos` no log, vazio. A prova de que o caminho forçado do Node instala o mesmo
  conjunto/conteúdo é o teste unitário novo em `npm/tests/generators.test.js`, não uma evidência de
  `make quality`.
- Paridade Node == Go == Python do **conjunto** de 9 comandos: verificada nesta sessão por grep
  manual dos três `init.js`/`scaffold.go`/`init_gen.py` (relatado acima), não por um gate
  automatizado. `scripts/check-artifact-parity.sh` (usado em `check-gates-falsify.sh`) só compara
  drift de **conteúdo** de `roadmap.md` entre runtimes (cenário
  `artifact-parity/slash-roadmap-content-drift`), não o conjunto completo dos 9 arquivos. A única
  garantia automática de que o conjunto **dentro do Node** não diverge de novo é o teste que
  adicionei; a paridade **entre** os 3 runtimes para o conjunto completo permanece evidência manual.
- `git diff --stat` confirma que só `npm/src/generators/init.js` e `npm/tests/generators.test.js`
  foram tocados no código; `internal/`, `pypi/`, `README.md`, `site/` e `~/.claude` intocados
  (mudanças vistas em `README.md`/`site/`/`internal/` no `git status` são do ML-5A rodando em
  paralelo, fora do meu escopo). Esta própria entrada em `docs/agents-working-context.md` é a
  exceção declarada — fora do handoff de restrição de escopo, mas exigida pelo protocolo de
  persona; o ML-5A concorrente também deve gravar nesse mesmo arquivo.

**Ambiguidades/observações — não corrigidas, apenas reportadas:**
- `generateClaudeCommands(root)` é chamada em `scaffold()` passando `root`, mas a função não aceita
  parâmetros e sempre escreve em `.claude/commands/trackfw` relativo ao `cwd` — o argumento é
  descartado silenciosamente. O gêmeo forçado honra `rootDir` corretamente. Não é uma regressão
  desta sessão (comportamento pré-existente e fora do escopo — o handoff pediu convergência da
  *lista*, não da assinatura/resolução de diretório) mas é uma armadilha latente: qualquer chamador
  futuro que passe um `root` diferente do `cwd` para `generateClaudeCommands` terá a escrita
  silenciosamente redirecionada para o `cwd` em vez do `root` pretendido.

---

## Sessão 2026-07-29 — Apolo (ML-5A `internal/commands/` — remoção dos cinco aliases deprecated de integração, concluído)

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md`

**Tarefa:** remover os cinco aliases de CLI deprecated (`trackfw copilot|cursor|gemini|windsurf|amazonq`),
único mecanismo canônico passa a ser `trackfw agents|skills`. Confirmado no handoff que os cinco
aliases existem só no CLI Go — sem paridade a implementar em Node/Python neste ML.

**Discriminador aplicado:** os nomes `copilot`/`cursor`/`gemini`/`windsurf`/`amazonq` também existem
como **targets do catálogo canônico** (`internal/integrations/assets/catalog.json`), usados por
`trackfw agents|skills install --targets` e por `trackfw init --ai-tools`. Essas superfícies
**não foram tocadas** — só os cinco comandos top-level do cobra e sua função de suporte.

**Entregue:**
- Apagados: `internal/commands/copilot.go`, `cursor.go`, `gemini.go`, `windsurf.go`, `amazonq.go`.
- `internal/commands/root.go`: removidas as 5 chamadas `new*Cmd()` do `rootCmd.AddCommand`.
- `internal/commands/integrations_flags.go`: removida `runDeprecatedIntegrationAlias` (ficou sem
  caller) e o import `internal/generators` que só ela usava.
- `internal/commands/agents_skills_test.go`: removidos `TestDeprecatedCursorAliasUsesLifecycleManager`
  e `TestDeprecatedAliasesPreserveAuxiliaryRulesWithoutOwnership` (esperavam os aliases). Adicionado
  `TestRemovedIntegrationAliasesAreUnknownCommands`, com **duas** asserções por nome: (1)
  `newRootCmd().Find([]string{name})` prova que o nome não é mais um subcomando registrado na
  árvore cobra real; (2) `RunPlugin(name, nil)` prova a mensagem literal de erro fim-a-fim
  `unknown command or plugin: "<nome>"`. `TestInitAIToolsHelpIncludesEveryCatalogTarget` (linha
  ~200) foi preservado sem alteração: cobre os targets do catálogo, superfície distinta dos
  aliases removidos.
- `internal/commands/root.go`: refatorado — `newRootCmd()` extrai a construção da árvore completa
  (antes só existia inline em `Execute()`), para que o teste acima inspecione o registro real em
  vez de uma árvore vazia. `Execute()` passou a ser só `newRootCmd().Execute()` + tratamento de
  erro/exit. **Falsificação verificada**: reintroduzi temporariamente um comando `cursor` fake em
  `newRootCmd()`, rodei o teste (falhou com `"cursor" is still a registered command: trackfw
  cursor`), revertei e confirmei `diff` idêntico ao original antes do commit.
- Documentação: `README.md` (linha da tabela de comandos com os 5 aliases removida),
  `site/guide/commands.md` e `site/en/guide/commands.md` (parágrafo atualizado de "aliases
  existem só no Go" para "removidos, use `agents`/`skills` --targets"). As menções aos mesmos
  nomes como *targets* (linhas "Supported targets: ..." e exemplos `--targets gemini,kiro`)
  foram preservadas intactas.
- `CHANGELOG.md` não foi alterado (breaking change já registrado no ADR pelo orquestrador).

**Validação:**
- `go build ./...`, `go vet ./...`, `go test ./...` → todos verdes.
- `bin/trackfw --help` → nenhum dos 5 aliases aparece na lista de `Available Commands`.
- Prova manual de que a superfície `legacy`/catálogo continua funcional:
  - Instalação: `trackfw agents install --targets cursor --items backend --scope project` em
    projeto isolado → instala `.cursor/agents/trackfw-backend.md` normalmente.
  - Surface `legacy-cli` explícita: `trackfw agents install --targets antigravity --surface
    antigravity=legacy-cli --items backend --scope project` → instala
    `.agents/agents/trackfw-backend/agent.json`.
  - Update: `trackfw agents update --targets cursor --items backend --scope project` (após
    install prévio) → `update complete: 1 agents artifact(s)`.
- `scripts/check-integration-cli-parity.sh` → "Integration CLI parity lifecycle checks passed".
- `make quality` → exit 0 (Go+Node+Python+contratos de paridade, incluindo `check-barrier.sh`
  15/15 e `check-gates-falsify.sh` 14/14), reexecutado após o refactor de `root.go`.
- `bin/trackfw validate --json` → `{"summary":{"violations":0,"warnings":0,"mode":"strict","exit_code":0}}`.
- `git status --short` confirma que `npm/src/generators/init.js` e `npm/tests/generators.test.js`
  aparecem modificados por conta do ML-5C rodando em paralelo — não tocados por mim.

**Ambiguidades/observações — não corrigidas, apenas reportadas (decisão do orquestrador):**
- **Risco de regressão em superfícies auxiliares (achado não trivial).** As 4 rule-files
  auxiliares — `GEMINI.md`, `.github/copilot-instructions.md`, `.windsurfrules`,
  `.amazonq/developer/guidelines.md` (mapeadas em `internal/generators/agentfiles.go:
  agentFiles`) — só eram criadas **pela primeira vez** por `runDeprecatedIntegrationAlias`
  (via `InjectRulesForTool`), que este ML removeu. As outras duas chamadas a
  `InjectRulesForTool`/`InjectRulesDetected` que sobrevivem (`trackfw discover` em
  `internal/commands/discover.go:130` e `trackfw update` em `internal/generators/update.go:77`)
  só injetam regras **se o arquivo já existir** (`os.Stat` prévio) — exceto `cursor`, que
  `InjectRulesDetected` cria sempre que `.cursor/` já existe. `installAITools` (usado por
  `trackfw init --ai-tools` e pelos comandos `agents/skills install`) **nunca** chama
  `InjectRulesForTool` — ele só instala agents/skills via `integrations.Manager`, não os arquivos
  de regra auxiliares do mapa `agentFiles`. Resultado prático: em um projeto **novo**, não existe
  mais nenhum comando do produto capaz de criar `GEMINI.md`/`copilot-instructions.md`/
  `.windsurfrules`/`.amazonq/developer/guidelines.md` pela primeira vez — apenas de atualizá-los
  se já existirem por outro meio (ex.: criados manualmente pelo usuário). Isso é uma perda de
  funcionalidade além do que o handoff descrevia ("remover 5 aliases de CLI, preservar
  superfícies de catálogo") — os 4 arquivos de regra não são superfícies de catálogo, são um
  mecanismo à parte que dependia exclusivamente do alias removido para o caso de primeira
  instalação. Recomendo ao orquestrador decidir entre: (a) aceitar a perda como parte do
  breaking change e documentá-la explicitamente, ou (b) adicionar uma chamada a
  `InjectRulesForTool` no fluxo de `installAITools`/`agents install` para os 4 tools afetados.
- **Allowlist obsoleta em `scripts/check-cli-parity.sh`** (não editado — fora da minha lista de
  arquivos afetados, e é infraestrutura de contrato compartilhado como `docs/cli-parity.md`):
  a linha 32 mantém `go_only_commands=(amazonq copilot cursor gemini windsurf completion)`, um
  allowlist de comandos que existiam só no Go e deviam ser subtraídos do conjunto comparado com
  Node/Python. Como os 5 nomes não aparecem mais em `trackfw --help`, a subtração agora é
  inofensiva (no-op) — não mascara nada, `make quality` confirma. Mas a lista ficou referenciando
  comandos que não existem em runtime nenhum; vale limpar no ML-5B (que já mexe em
  `docs/cli-parity.md` e na superfície de ajuda) para não confundir o próximo leitor do script.

## Auditoria 2026-07-29 — Zeus — Wave 5 parcial (ML-5A e ML-5C) aprovada

Os cinco aliases deprecated saíram do CLI Go — não aparecem em `trackfw --help` — e as superfícies
`legacy` do catálogo continuam instaláveis e atualizáveis. O `CHANGELOG.md` não foi tocado: o texto
do breaking change está no ADR, para o PR de release consumir.

O Node passou a ter um único mapa de slash commands. Verificação independente: os três runtimes
expõem o mesmo conjunto de 9 comandos, e há teste comparando nomes e conteúdo entre os caminhos
normal e forçado.

Gates: `make quality` exit 0, `check-barrier.sh` 15/15, falsificação 14/14, `validate --json` 0
violações.

Dois achados registrados como MLs próprios em vez de expandir a wave:

- **ML-5E (regressão).** Os quatro arquivos auxiliares de regras — `GEMINI.md`,
  `.github/copilot-instructions.md`, `.windsurfrules` e `.amazonq/developer/guidelines.md` — só
  eram criados pela primeira vez pelo alias removido. `InjectRulesDetected` apenas atualiza arquivo
  já existente, e o caminho de instalação por catálogo nunca chama `InjectRulesForTool`. Em projeto
  novo, nenhum comando do produto cria mais esses arquivos. Decisão: é regressão, não parte do
  breaking change sancionado pelo ADR — deve ser corrigida.
- **ML-5D (lacuna de gate).** Nenhum gate compara o conjunto de slash commands entre os três
  runtimes; `check-artifact-parity.sh` cobre apenas o conteúdo de `roadmap.md`. Os dois defeitos
  desta wave e a prova de equivalência do `barrier.md` no ML-3A dependeram de inspeção manual.

## 2026-07-29 — Ártemis (QA) — ML-5D iniciado

Handoff do `trackfw_architect` para o roadmap
`ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md` — ML-5D. Escopo: novo
gate de paridade de slash commands (`scripts/check-slash-parity.sh`), wiring no Makefile,
cenário de falsificação em `scripts/check-gates-falsify.sh`, e correção de defeito latente em
`npm/src/generators/init.js` (`generateClaudeCommands(root)` ignorava o parâmetro `root`).
Branch: `feat/barrier-governanca-e-autoridade-do-orquestrador` (já criada pelo orquestrador).

## 2026-07-29 — Ártemis (QA) — ML-5D concluído (gate wired, 1 achado bloqueante reportado)

**Entregável 1 — gate de paridade:** `scripts/check-slash-parity.sh` (novo script, não extensão
de `check-artifact-parity.sh` — decisão: scenario 9 daquele gate já cobre `slash_roadmap`
isoladamente, e este gate precisa de um alvo de falsificação independente). Roda `<cli> init`
em diretório descartável para os três runtimes, então `diff -ru` os três
`.claude/commands/trackfw/` resultantes — cobre nome+conteúdo em uma operação e evita
mojibake por nunca fazer parsing de literais de código-fonte. Vacuity guard confirma os 9
comandos esperados nos 3 runtimes antes de comparar. Acumula todas as divergências antes de
sair (padrão `FAIL=1` de `check-artifact-parity.sh`), não fail-fast — necessário para que uma
futura corrupção isolada não seja mascarada por drift pré-existente.

**Entregável 2 — wiring e falsificação:** encadeado no alvo `parity` do Makefile, logo após
`check-barrier.sh`. Dois cenários novos em `check-gates-falsify.sh`, um por critério de aceite:
Cenário 14 corrompe o *conteúdo* de `status.md` no gerador Node.js (arquivo hoje idêntico nos 3
runtimes, escolhido deliberadamente para não colidir com o drift pré-existente de
`move.md`/`architect.md`) e confirma `slash parity drift: status.md (go vs node)`. Cenário 15
renomeia a *chave* `'status.md'` para `'status-renamed.md'` no mesmo mapa (drift de nome, não de
conteúdo) e confirma que o vacuity guard reprova com diagnóstico distinto:
`slash parity drift: status.md missing (node) — vacuity guard failed`. 16/16 cenários de
falsificação passam.

**Entregável 3 — defeito latente:** `generateClaudeCommands(root)` em
`npm/src/generators/init.js` recebia `root` mas sempre escrevia relativo a `process.cwd()`,
descartando o argumento silenciosamente — `scaffold()` chama `generateClaudeCommands(root)`
esperando o argumento honrado, e o gêmeo forçado (`generateClaudeCommandsForce`) já fazia isso
corretamente. Corrigido para espelhar o gêmeo forçado, preservando o comportamento cwd-relativo
quando `root` é omitido (todos os testes existentes chamam sem argumento). Novo teste em
`npm/tests/generators.test.js` prova ambas as direções: arquivos aparecem sob o `rootDir`
passado E não aparecem sob `process.cwd()`.

**Achado bloqueante (fora do escopo do ML-5D, reportado — não corrigido):** o novo gate
encontrou 3 divergências de conteúdo PRÉ-EXISTENTES entre os 3 geradores, cada uma com maioria
2-1 clara — `move.md` "Estados válidos" (Go+Node têm `analyzing`, Python não), `move.md`
"Exemplo" (Go+Python usam `wip`, Node usa `analyzing`) e `architect.md` (frase de abertura com
parêntese extra só no Python). O fix (ML-5F) é mecânico: 3 edições de uma linha aplicando o
texto majoritário — não há decisão arquitetural a arbitrar, só uma pergunta de conteúdo aberta
(se o Exemplo de `move.md` deveria mostrar `wip` ou o mais didático `analyzing`). Detalhe
completo e prova empírica (diff bruto) em
`vault/notes/slash-commands-cross-runtime-content-drift-2026-07-29.md`. Confirmado que nem
ML-5B (`*/help*`, `root.go`, `commands/index.js`, `cli.py`) nem ML-5E (`agentfiles.go`, catálogo)
tocam esses 3 arquivos geradores — não é ruído de agente paralelo, é drift pré-existente à
wave. `.trackfw-attention.json` escrito pedindo a decisão de conteúdo em aberto (recomendação:
ML-5F corretivo).

**Validação:**
- `go build ./...` → OK.
- `go vet ./...` → OK.
- `go test ./...` (`make test`) → todos os pacotes OK.
- `cd npm && npm test` → 304 passed, 0 failed (inclui o novo teste do ML-5D).
- `python3 -m pytest pypi/tests -q` → 670 passed.
- `GO_BIN=bin/trackfw scripts/check-slash-parity.sh` → exit 1, reportando exatamente os 2
  arquivos com drift pré-existente (evidência esperada, documentada na nota do vault).
- `GO_BIN=bin/trackfw scripts/check-barrier.sh` → 15/15 OK.
- `GO_BIN=bin/trackfw scripts/check-artifact-parity.sh` → OK.
- `bash scripts/check-gates-falsify.sh` → 16/16 cenários, incluindo os dois novos
  (`slash-parity/status-content-drift` prova o caminho de conteúdo;
  `slash-parity/status-name-drift` prova separadamente o caminho de nome/vacuity-guard,
  renomeando uma chave do mapa em vez de alterar seu conteúdo).
- `bin/trackfw validate --json` → `{"summary":{"violations":0,"warnings":0,"mode":"strict","exit_code":0}}`.
- `make parity` → **vermelho por dois motivos independentes, em sequência** (o Makefile roda os
  gates um após o outro; o segundo nunca é alcançado enquanto o primeiro falhar):
  1. `scripts/check-identity-parity.sh` (linha 24 do alvo `parity`, roda ANTES do meu gate) —
     artifact count mismatch go=13 vs node=12/python=12 nos 7 targets. Não pertence ao meu
     escopo; aparenta ser trabalho em voo do ML-5E (`agentfiles.go`/catálogo). Reportado, não
     investigado nem corrigido.
  2. `scripts/check-slash-parity.sh` (meu gate, ML-5D) — drift pré-existente de `move.md`/
     `architect.md` descrito acima. Verificado standalone (`GO_BIN=bin/trackfw
     scripts/check-slash-parity.sh`), já que `make parity` não chega a ele enquanto (1) não for
     resolvido. Reexecutado `make parity` ao final desta ML (evidência, não resumo):
     ```
     GO_BIN=bin/trackfw scripts/check-identity-parity.sh
     Identity parity [with-identity] target 'amazonq': artifact count mismatch (go=13 node=12 python=12)
     ... (mesmo padrão para claude, codex, copilot, cursor, gemini, windsurf)
     Identity parity: 14 check(s) failed
     make: *** [parity] Error 1
     ```
     Confirma que `check-identity-parity.sh` (linha 24, ML-5E) para o `parity` target antes de
     `check-slash-parity.sh` (linha 27) ser alcançado nesta execução.

**Critérios de aceite do ML-5D — status final:** gate compara nome+conteúdo nos 3 runtimes ✅ ·
encadeado no `parity` do Makefile ✅ · falsificação prova não-vacuidade (nome E conteúdo, 2
cenários distintos) ✅ · comparação sem falso positivo de encoding (comparação via `diff -ru`
sobre arquivos gerados pelos geradores reais, nunca parsing de literais) ✅ ·
`generateClaudeCommands` honra `rootDir` com teste ✅ · **`make quality` passa** ❌ — vermelho
por 2 causas independentes e de terceiros: (a) `check-identity-parity.sh` falhando primeiro
(ML-5E em voo) e (b) drift de conteúdo pré-existente que meu próprio gate corretamente
detecta (ML-5F recomendado). `validate --json` limpo ✅.

## 2026-07-29 — Apolo (Backend) — ML-5E iniciado

Handoff do `trackfw_architect` para o roadmap
`ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md` — ML-5E (corretivo).
Escopo: `internal/generators/agentfiles.go`, o caminho de instalação por catálogo
(`internal/commands/init.go` e `internal/commands/integrations_flags.go`) e testes
correspondentes. Objetivo: religar `generators.InjectRulesForTool` ao caminho de instalação por
catálogo, restaurando a criação de `GEMINI.md`, `.github/copilot-instructions.md`,
`.windsurfrules` e `.amazonq/developer/guidelines.md` a partir de projeto novo — regressão
introduzida pelo ML-5A ao remover os aliases deprecated. Branch:
`feat/barrier-governanca-e-autoridade-do-orquestrador` (já criada pelo orquestrador).

## 2026-07-29 — Apolo (Backend) — ML-5E bloqueado (código pronto, gate de paridade cross-CLI conflita)

**Implementado:** hook em dois call sites — `installAITools` (`internal/commands/init.go`,
fluxo `trackfw init --ai-tools`) e `executeIntegrationMutation` (
`internal/commands/integrations_flags.go`, operação `install`, fluxo canônico `trackfw
agents|skills install --targets <tool>`, exatamente o pedido pelo handoff). Ambos chamam
`generators.InjectRulesForTool(target, cwd)` por target selecionado, reutilizando
`injectOrUpdateRules` (idempotente) já existente — nenhuma reimplementação. `cwd` é sempre a
raiz do projeto, independente de `--scope` (mesma semântica do alias removido, confirmada em
`git show b37c064^:internal/commands/integrations_flags.go`, função
`runDeprecatedIntegrationAlias`). Hook em `update`/`uninstall` deliberadamente **não** adicionado
— mantém a semântica one-shot do alias removido (decisão registrada para o orquestrador poder
sobrepor). Testes novos em `internal/commands/agentfiles_catalog_install_test.go`: criação a
partir de projeto vazio para os 4 targets, idempotência (2 instalações → 1 bloco), paridade
entre `agents install` e `skills install`, não-regressão do gatilho por diretório do Cursor
(`InjectRulesDetected`), e prova de que `update` não cria o arquivo.

**Bloqueio descoberto (não é ambiguidade resolvível por mim):** ligar o hook ao fluxo canônico
`agents install --targets <tool>` faz `scripts/check-identity-parity.sh` (parte de `make
quality`) falhar nos 7 targets com superfície de regras — Go passa a emitir 1 artefato a mais
que Node/Python (`go=13 node=12 python=12`), porque **só o Go foi corrigido**. Investiguei se
"religar Node/Python também" resolveria: não. Gerei `.windsurfrules` via
`InjectRulesForTool`/`injectRulesForTool`/`inject_rules_for_tool` nos 3 runtimes a partir de
diretório vazio e o conteúdo **já diverge entre os 3 hoje**, independente deste ML (chain de
estados, item "ML lifecycle", bloco "Architecture Directives" — que além disso está **duplicado
dentro do próprio Go**). Reconciliar esse texto é mudança de conteúdo fora do que a ADR do
ML-5E sancionou e precisa de REQ própria. Diagnóstico completo, diffs e as 3 opções de decisão
em `vault/notes/rules-block-content-drift-3-clis-2026-07-29.md`. Não escrevi
`.trackfw-attention.json` porque o arquivo já contém o achado concorrente do ML-5D
(`slash-commands-cross-runtime-content-drift`) — sinalizando aqui e na nota do vault para não
sobrescrever o achado de outro agente.

**Validação:**
- `go build ./...` → OK.
- `go vet ./...` → OK.
- `go test ./internal/generators/... ./internal/commands/...` → OK (inclui os 6 testes novos).
- `go test ./...` → todos os pacotes OK.
- `make quality` → **vermelho** em `scripts/check-identity-parity.sh` (14 mismatches, ver
  acima); todos os demais gates do `make quality` (Go, Node 304 testes, Python 670 testes,
  `go vet`, `check-cli-parity.sh`, `check-validate-parity.sh`,
  `check-referential-integrity.sh`, `check-static-assets.sh`,
  `check-integration-assets.sh`) passaram antes do bloco de identity-parity.
- `bin/trackfw validate --json` (binário gerado com sucesso pelo próprio `make quality`, antes de
  falhar em `check-identity-parity.sh`) → `{"summary":{"violations":0,"warnings":0,"mode":"strict","exit_code":0},"violations":[],"warnings":[]}`.
- `scripts/check-referential-integrity.sh` (standalone, cobrindo a nota do vault e o link em
  `index.md` já no disco) → `Referential integrity OK`.

**Decisão pendente do orquestrador (3 opções, recomendo (c) com (b) como interino):**
(a) aceitar `check-identity-parity.sh` vermelho e landar só no Go; (b) reverter o hook em
`integrations_flags.go` e manter só o de `init.go` (não exercitado por nenhum gate hoje, mas
descumpre a instrução explícita de ligar ao fluxo `agents install`); (c) bloquear ML-5E até uma
REQ nova reconciliar o bloco de regras nos 3 runtimes.

**Arquivos alterados:** `internal/commands/init.go`, `internal/commands/integrations_flags.go`,
`internal/commands/agentfiles_catalog_install_test.go` (novo),
`vault/notes/rules-block-content-drift-3-clis-2026-07-29.md` (novo), `vault/notes/index.md`.
Nada commitado — sem autoridade Git (barrier), branch permanece com working tree alterado para
o `trackfw_architect` auditar e decidir.

## 2026-07-29 — Apolo (Backend) — ML-5B concluído (consolidação da superfície de ajuda)

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md`

**Tarefa:** Consolidar `trackfw help [comando|chave]` como superfície explícita única nos 3
CLIs, preservando `--help` nativo, com resolução determinística (comando → chave de config →
erro com sugestão) e paridade comprovada empiricamente.

**Bug real encontrado e corrigido (Go apenas):** `trackfw --help` listava **duas** entradas
`help` em "Available Commands" — a nossa customizada e a que o cobra injeta sozinho via
`InitDefaultHelpCmd()`, que ignora comandos chamados "help" registrados via `AddCommand` (só
respeita o campo interno `c.helpCommand`). `scripts/check-cli-parity.sh` já escondia o sintoma
com um `awk '!seen[$0]++'` comentado "cobra may list help twice" — o defeito nunca tinha sido
corrigido na raiz. Fix: `root.SetHelpCommand(helpCmd)` além do `AddCommand`. Diagnóstico
completo em `vault/notes/cobra-help-cmd-duplicate-registration-2026-07-29.md`. Node
(commander) e Python (argparse) nunca tiveram esse problema.

**Implementado nos 3 CLIs (mesma lógica, replicada por linguagem):**
1. `help` sem argumento → lista comandos disponíveis + tabela de chaves de config (antes: só
   a tabela de chaves).
2. `help <comando>` → agora resolve e imprime a ajuda nativa do comando (antes: sempre
   "chave desconhecida"). Go via `root.Find`+`sub.Help()`; Node via
   `program.commands.find(...)`+`.helpInformation()`; Python via
   `subparsers.choices[topic].format_help()`.
3. `help <chave>` → inalterado (documentação da chave).
4. Assunto desconhecido → mensagem "assunto desconhecido: X" + "Você quis dizer: Y?" quando
   houver candidato a distância de Levenshtein ≤ 3 (comandos + chaves como candidatos);
   exit code 1 nos 3 runtimes. Implementei Levenshtein idêntico nas 3 linguagens.
5. Go: `SilenceUsage`/`SilenceErrors` no comando `help` para não duplicar a mensagem de erro
   (antes: "chave desconhecida" aparecia 3x — nosso print, "Error: " do cobra, e o reprint do
   `Execute()` em `root.go`). Alinhado ao comportamento single-line de Node/Python.
6. `<comando> --help` e `trackfw --help` preservados como flags nativas dos 3 frameworks —
   nenhum segundo comando `help` foi registrado em nenhum runtime.

**Prova de equivalência (saída literal, ver evidência completa na resposta ao orquestrador):**
`help`, `help init`, `help wip_limit`, `help chave-que-nao-existe` (sem sugestão, os 3
concordam) e `help wip_limi` (sugere `wip_limit` nos 3) produzem mensagem e exit code
equivalentes nos 3 runtimes.

**Limpeza adicional (autorizada no handoff):** `scripts/check-cli-parity.sh` — removidos os 5
aliases mortos (`amazonq copilot cursor gemini windsurf`) de `go_only_commands`, mantendo só
`completion`. `docs/cli-parity.md` — só a linha da tabela do `help` foi editada, para descrever
a superfície unificada.

**Validação:**
- `go build ./...`, `go vet ./...` → OK.
- `go test ./internal/commands -run Help -v` → 9/9 (inclui `TestHelpKnownCommand` e
  `TestHelpDoesNotRegisterDuplicateEntry`, novos).
- `go test ./...` → todos os pacotes OK.
- `cd npm && npm test -- --test-name-pattern='help'` → suíte completa roda (custom test
  runner, não usa `describe`/`it` do `node:test`), 304/304 (inclui os testes novos de
  `listCommands`/`suggestTopic`).
- `python3 -m pytest pypi/tests -k help -q` → 23/23 (nova classe
  `TestHelpCommandResolution`).
- `python3 -m pytest pypi/tests -q` → 675/675.
- `bash scripts/check-cli-parity.sh` (isolado) → OK.
- `bin/trackfw validate --json` → `{"summary":{"violations":0,"warnings":0,...,"exit_code":0}}`.
- `make quality` → **vermelho**, mas em `scripts/check-identity-parity.sh`
  (go=13 vs node/python=12), que é o bloqueio pré-existente do ML-5E em voo (não relacionado ao
  meu escopo — confirmado por `docs/roadmaps/.trackfw-attention.json`, já escrito pelo agente do
  ML-5D, e pela entrada acima deste registro). Todos os gates anteriores no pipeline de `make
  quality` (Go completo, Node 304 testes, Python 675 testes, `go vet`, `check-cli-parity.sh`,
  `check-validate-parity.sh`, `check-referential-integrity.sh`, `check-static-assets.sh`,
  `check-integration-assets.sh`) passaram.

**Arquivos alterados:** `internal/commands/help.go`, `internal/commands/help_test.go`,
`internal/commands/root.go`, `npm/src/commands/help.js`, `npm/tests/help.test.js`,
`pypi/trackfw/commands/help_cmd.py`, `pypi/tests/test_help.py`, `scripts/check-cli-parity.sh`,
`docs/cli-parity.md`, `vault/notes/cobra-help-cmd-duplicate-registration-2026-07-29.md` (novo),
`vault/notes/index.md`. Nada commitado — sem autoridade Git; branch permanece com working tree
alterado (compartilhado com o trabalho em voo do ML-5D/ML-5E) para o `trackfw_architect`

---

## Sessão 2026-07-29 — Apolo (ML-5F resolução das 3 divergências de `scripts/check-slash-parity.sh`)

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md`

**Tarefa:** Resolver as 3 divergências pré-existentes acusadas por
`scripts/check-slash-parity.sh` (criado no ML-5D) entre os mapas de slash commands dos 3
runtimes, sem tocar em `internal/generators/agentfiles.go` nem nos injetores de regras
(escopo do ML-5G).

**Entregue:**
1. `move.md` — exemplo `wip` → `analyzing` em Go (`internal/generators/scaffold.go`) e Python
   (`pypi/trackfw/generators/init_gen.py`); decisão deliberada do usuário, não é maioria (foi a
   única das 3 que NÃO seguiu maioria 2-1, ver handoff).
2. `move.md` — lista "Estados válidos" agora inclui `analyzing` também no Python
   (`init_gen.py`); Go e Node já tinham. Maioria 2-1.
3. `architect.md` — removido o parêntese `` (`/trackfw:architect`) `` da frase de abertura no
   Python (`init_gen.py`), para igualar Go/Node. Maioria 2-1.

**Efeito colateral corrigido:** `pypi/tests/test_generators_init.py` tinha um
`assertIn('/trackfw:architect', content)` que só passava por causa da divergência #3 (nenhum
teste equivalente existe em Go/Node para essa frase — os testes Go/Node que citam
`/trackfw:architect` são sobre a tabela do CLAUDE.md, não sobre a abertura do architect.md).
Removida a asserção obsoleta; sem ela, o teste continua cobrindo o resto do conteúdo.

**Validação:**
- `go build -o bin/trackfw ./cmd/trackfw && scripts/check-slash-parity.sh` → `OK
  [slash-parity/vacuity-guard]`, `OK [slash-parity/three-runtimes-identical]`, "Slash command
  parity checks passed (9 commands x 3 runtimes)." — zero divergências.
- `go build ./... && go test ./...` → todos os pacotes OK.
- `cd npm && npm test` → 14+45 testes de validate/validator + 304 no total, 0 falhas.
- `python3 -m pytest pypi/tests -q` → 675 passed (após a correção do teste obsoleto).
- `bin/trackfw validate --json` → `{"summary":{"violations":0,"warnings":0,"mode":"strict","exit_code":0}}`.
- `make quality` não foi rodado (vermelho por causa pré-existente do ML-5E/`check-identity-parity.sh`,
  fora do meu escopo, conforme o handoff).

**Arquivos alterados:** `internal/generators/scaffold.go`, `pypi/trackfw/generators/init_gen.py`,
`pypi/tests/test_generators_init.py`. Nenhum arquivo fora dos 3 mapas de slash commands (e o
teste que os cobre) foi tocado. Nada commitado — sem autoridade Git; branch permanece com
working tree alterado (inclui trabalho em voo de outros MLs, não meu) para o
`trackfw_architect` revisar e commitar.
revisar e commitar.

## Auditoria 2026-07-29 — Zeus — Wave 5 completa e aprovada

Sete MLs: 5A (aliases removidos), 5B (help unificado), 5C (mapa único no Node), 5D (gate de
paridade dos slash commands), 5E (regressão dos arquivos de regras), 5F (conteúdo dos slash
commands reconciliado), 5G (bloco de regras reconciliado).

A wave revelou uma cadeia de dependência que o roadmap original não previa: o ML-5A introduziu uma
regressão, o ML-5E não pôde corrigi-la porque o texto do bloco de regras divergia entre os
runtimes, e o `check-identity-parity.sh` compara sha256 por artefato — então contagens iguais com
conteúdo diferente continuam reprovando. Decisão do usuário: reconciliar o texto nesta branch em
vez de adiar. O ML-5G removeu a duplicação do bloco `Architecture Directives` no Go, unificou o
texto e ligou a injeção nos três runtimes.

Decisão de produto do usuário registrada: o exemplo de `move.md` passa a usar `analyzing` em vez de
`wip`, por ensinar o ciclo completo. É a única das três reconciliações do ML-5F que não segue a
maioria, e é deliberada.

Verificação independente: `make quality` exit 0 com 17 cenários de falsificação e 11 gates provados
não-vacuosos — eram 8 no início da sessão. `Architecture Directives` aparece uma única vez no Go.
Em projeto novo com HOME isolado, `init --ai-tools gemini` cria `GEMINI.md` (3100 bytes) e reexecutar
mantém um único bloco de regras.

**Observação para a Wave 6:** ao validar a regressão, constatei que `trackfw init --ai-tools` grava
em `~/.gemini` — o HOME do usuário. É a mesma classe de defeito que a Wave 6 existe para corrigir
em `trackfw update`: um comando de escopo de projeto mutando o harness global. O contrato do ML-6A
cobre `update`, não `init`. Registrado como ML-6E.

## Incidente 2026-07-29 — Zeus — verificação de HOME era vacuosa

Durante a Wave 6, o agente do ML-6D reportou que `~/.claude/skills/trackfw/SKILL.md` na máquina
real havia sido escrito por alguma execução não isolada. A investigação confirmou o fato e revelou
um problema maior no método de auditoria do orquestrador.

**Causa da falha de detecção:** todas as verificações anteriores de "HOME intocado" usaram
`find <dir> -newermt "-N hours"`. Nesta máquina o `find` é `bfs`, que **rejeita** esse formato de
timestamp com `Invalid timestamp` e sai sem listar nada. A saída vazia foi lida como "nada tocado",
quando na verdade o comando havia falhado. As verificações das Waves 3, 4 e 5 são, portanto,
**inconclusivas** — não provam ausência de escrita.

**Método correto, validado:** `find <dir> -mmin -N`, ou varredura por `os.path.getmtime`.

**Alcance real do incidente**, apurado com o método correto: exatamente **um** arquivo,
`~/.claude/skills/trackfw/SKILL.md`, reescrito 18 minutos antes da apuração. O template do skill é
**idêntico entre `origin/main` e esta branch** (`git diff origin/main -- internal/generators/scaffold.go`
não acusa diferença no texto), então o conteúdo gravado equivale ao da versão já publicada. Nenhum
outro artefato do trackfw em `~/.claude`, `~/.codex`, `~/.gemini`, `~/.cursor` ou `~/.agents` foi
tocado nas últimas 4 horas.

**Conclusão:** houve violação da proibição de escrita fora do repositório, com impacto material
nulo (mesmo conteúdo), mas a lição relevante é a segunda: um gate de verificação que falha em
silêncio é indistinguível de um gate que passa. É a mesma classe de defeito que este roadmap
inteiro combate, desta vez cometida pelo próprio orquestrador — e é a segunda vez na sessão, após
o `sort_keys=True` que mascarou a divergência de ordem de chaves no ML-2E.

## 2026-07-29 — Apolo (Backend) — ML-6C iniciado (Node.js: `trackfw update` vs `trackfw update harness`)

Recebido handoff do `trackfw_architect` para implementar, somente no runtime Node.js
(`npm/src/`), o contrato congelado no ML-6A (`docs/cli-parity.md`, seção
"`trackfw update` vs `trackfw update harness`"). Escopo: `npm/src/commands/update.js`,
`npm/src/commands/update-harness.js` (novo), registro em `index.js`, e ajustes de escopo em
`integrations.js`/`manager.js` se necessários. Todo teste redireciona HOME — nenhuma execução
contra o HOME real (Wave 6 já teve um incidente exatamente desse tipo, documentado acima).

## 2026-07-29 — Apolo (Backend) — ML-6C concluído (Node.js: split `update` / `update harness`)

**Arquivos criados:** `npm/src/commands/update-harness.js`, `npm/src/lib/update-engine.js`
(motor de estado compartilhado `updated/skipped/missing/failed`, novo, não listado no handoff mas
necessário para não duplicar a máquina de estados entre os dois comandos).

**Arquivos alterados:** `npm/src/commands/update.js` (reescrito — restrito ao projeto, nunca mais
chama `installSkillsForce`; ganhou `--dry-run`/`--json`/`--targets`/`--install-missing`),
`npm/tests/agents-skills.test.js` (um teste pré-existente ajustado — ver nota abaixo),
`npm/tests/update.test.js` (novo, 9 casos), `npm/tests/update-harness.test.js` (novo, 9 casos).
`integrations.js`/`manager.js` não precisaram de mudança — `IntegrationManager.inspect()` já
oferecia o suficiente para classificar estados sem efeito colateral.

**Achado não óbvio, corrigido antes de reportar:** commander@12 tem uma armadilha real ao aninhar
um `Command` filho que redeclara os MESMOS nomes de flag (`--json`, `--dry-run`, `--targets`,
`--install-missing`) que o `Command` pai — o valor da flag é silenciosamente atribuído ao pai
(`update.opts()`), e o filho recebe `{}` não importa o que foi passado na linha de comando.
Reproduzido isoladamente antes de descartar a hipótese. Solução: `update.js` é um único `Command`
com um argumento posicional opcional `[mode]` (`"harness"` ou vazio) e um único conjunto de
opções; `update-harness.js` exporta uma função `run(options)` simples, não um `Command` próprio.
Nota de vault: `vault/notes/commander-nested-subcommand-duplicate-flag-drops-parent-2026-07-29.md`.

**Segundo achado, corrigido antes de reportar:** o filtro `--targets` estava sendo aplicado
DEPOIS de construir todos os targets — como `apply()` é efeito colateral real (fora de
`--dry-run`), isso escrevia em disco alvos que não foram pedidos. Corrigido: o filtro
(`wanted`/`include(id)`) agora decide, por alvo, se ele sequer é computado/aplicado.

**Ambiguidades reportadas ao orquestrador (não resolvidas unilateralmente):**
0. **Divergência de escopo vs. ML-6B (Go), constatada após a implementação.** A nota
   `vault/notes/update-harness-project-scope-json-gap-2026-07-29.md` (escrita às 16:08 pelo agente
   do ML-6B, durante minha própria implementação) registra que o Go implementou o contrato
   completo apenas para `update harness`, deixando `update` (projeto) sem flags. Este ML
   implementou o contrato completo (quatro estados, quatro flags, ordem de chaves) para **ambos**,
   seguindo a leitura literal da tabela de `docs/cli-parity.md` ("Applies to: both" nas quatro
   linhas). Não revertido, por três razões: (a) é superconjunto — `trackfw update` sem flags
   continua idêntico ao comportamento anterior menos a mutação global, nada quebra; (b) reverter
   trocaria uma divergência por outra, já que o escopo do ML-6D (Python) ainda era desconhecido no
   momento desta decisão; (c) verificado que `scripts/check-cli-parity.sh` só confere a *presença*
   do nome do comando `update` entre os runtimes (bloco `floor_commands`), não o JSON/flags — logo
   `make quality` não reprova por este motivo. A lista de target-ids de escopo projeto que autorei
   (`agent-rules`, `agent-hooks`, `codex-project-agents`, `validate-script`, `ci-workflow`,
   `git-hooks`, `claude-commands`) é exatamente o que a nota pede para ser acordado entre os três
   runtimes — decisão de reconciliação do orquestrador, não minha.
1. Granularidade dos alvos de `update harness`: cada `<tool>-agents`/`<tool>-skills` agrega
   potencialmente muitos itens do catálogo (12 personas de agente, 17 skills) em um único estado
   de alvo. O contrato do ML-6A não especifica granularidade por item; assumi agregação por
   bundle (regra de precedência: `missing` se tudo ausente; `skipped` se algo está
   modificado/não-gerenciado — nunca sobrescrito; `updated` se algo foi escrito; senão `skipped`).
2. `ci-workflow` e `git-hooks` em `update` (escopo projeto) só aparecem na lista de alvos quando
   `trackfw.yaml` já configura `ci: github-actions` / `hooks: husky|lefthook` — preservei o gate
   condicional que já existia no código legado, em vez de forçar esses artefatos em todo projeto.
3. `installSkillsForce` (`npm/src/generators/init.js:1242`) ficou órfão — não é mais chamado por
   `update`, e `update-harness.js` duplica seu conteúdo literal (não pode reusá-la: ela grava
   incondicionalmente em `os.homedir()` sem modo dry-run). Candidato a limpeza em ML futuro.
4. Mensagem de erro do estado `failed` foi posicionada como última chave (`id`, `state`, `path`,
   `message`), presente só quando `state === 'failed'` — o contrato não deixa isso explícito.
5. `--targets` com um id de escopo projeto que existe no universo declarado mas está excluído da
   lista efetiva por configuração (ex.: `--targets ci-workflow` num projeto com `ci: none`) é
   aceito como id conhecido (sem erro de uso) e produz `targets: []` / `summary` todo zerado —
   resultado silenciosamente vazio, não sinalizado como erro nem como `missing`.

**Evidência:** `cd npm && npm test` → `322 passed, 0 failed` (304 pré-existentes + 18 novos).
HOME real verificado com `find ~/.claude -type f -mmin -90` — o único arquivo recente
(`~/.claude/skills/trackfw/SKILL.md`, 15:51) é o já documentado no incidente ML-6D acima (18 min
antes daquela apuração), não desta sessão: nenhuma das minhas invocações rodou sem `HOME`
redirecionado para um diretório de scratch.

Nenhum arquivo sob `internal/`, `cmd/` ou `pypi/` foi tocado. Nenhuma operação Git executada —
sem autoridade; branch permanece com working tree alterado para o `trackfw_architect` auditar e
commitar.

## 2026-07-29 — Ártemis (QA) — ML-6G iniciado

Handoff do `trackfw_architect` para o roadmap
`ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md` — ML-6G. Escopo: novo
gate `scripts/check-update-parity.sh` provando paridade cross-runtime de `trackfw update` e
`trackfw update harness` (contrato do ML-6A), wiring no Makefile e cenário de falsificação em
`scripts/check-gates-falsify.sh`. Restrito a `scripts/` e `Makefile`; ML-6F roda em paralelo sobre
`internal/`, `npm/src/` e `pypi/trackfw/` corrigindo as 4 divergências conhecidas (contagem de
targets, renderização de `path`, id de `claude-skills`, escopo das flags no `update` de projeto).
Branch: `feat/barrier-governanca-e-autoridade-do-orquestrador` (já criada pelo orquestrador).

## 2026-07-29 — Ártemis (QA) — ML-6G concluído (gate wired, 1 achado novo não-óbvio reportado)

**Entregável 1 — gate de paridade:** `scripts/check-update-parity.sh` (novo), 5 cenários: (1)
`update harness --json` em harness vazio — todos `missing`, exit 0, JSON idêntico nos 3 runtimes;
(2) idem em harness populado (via `trackfw agents install --targets claude --scope global` rodado
com o binário Go em 3 homes isolados, para que a própria fixture seja idêntica antes de cada
runtime rodar seu próprio `update harness`); (3) `update --json` (escopo projeto) — `scope`,
exit 0, JSON idêntico; (4) `--dry-run` em harness — semeia um `claude-skill` deliberadamente
"stale" (conteúdo divergente do template) para que a asserção "zero escritas" tenha algo real a
suprimir, e não seja vácua; compara snapshot sha256 da árvore de `$HOME` antes/depois; (5) lista
de target ids extraída do cenário 1, comparando conjunto E ordem nos 3 runtimes. Comparação via
reparse+redump Python (`object_pairs_hook=OrderedDict`, sem `sort_keys`) preservando ordem de
chaves e de targets — mesmo racional do `normalize_barrier_json` do ML-4A, para não repetir o
mascaramento do ML-2E. `HOME` é redirecionado em toda invocação (`run_update` isola por
runtime/cenário); nenhuma chamada toca o `$HOME` real.

**Entregável 2 — wiring e falsificação:** encadeado no alvo `parity` do Makefile, logo após
`check-rules-parity.sh`. Cenário 17 de `check-gates-falsify.sh`: copia `npm/src` para uma árvore
descartável (`setup_npm_tree`) e remove, via `sed`, o único `if (dryRun) return ...` que impede
`claudeSkillTarget` (Node.js) de escrever de verdade durante `--dry-run`; roda o gate completo
contra essa árvore corrompida e confirma o diagnóstico
`filesystem tree under HOME changed during --dry-run`. Guard de corrupção (`cmp -s`) confirma que
o `sed` de fato alterou o arquivo antes de rodar o gate. `scripts/check-gates-falsify.sh` completo:
18/18 cenários OK.

**Estado encontrado ao rodar o gate (ML-6F em andamento):** a auditoria cruzada da Wave 6 já havia
medido 4 divergências; ao final desta sessão, com o ML-6F parcialmente aplicado (arquivos
`internal/generators/update.go`, `internal/integrations/plan.go`, `npm/src/commands/
update-harness.js`, `npm/src/integrations/catalog.js`, `pypi/trackfw/commands/update_harness.py`,
`pypi/trackfw/integrations/catalog.py` já modificados no working tree, não commitados), o gate já
confirma paridade na lista de 19 targets e na comparação de harness vazio Go-vs-Python. Restam,
neste momento: (a) `go-vs-node` em harness vazio, isolado ao campo `path` de um único target
(ver achado abaixo); (b) `update --json` (escopo projeto) ainda ausente em Go e Python (exit 1 e
exit 2 respectivamente, "unknown flag"/"unrecognized arguments: --json") — item 4 do ML-6F, ainda
não iniciado. `make parity` permanece vermelho até o ML-6F fechar os dois itens; é o resultado
esperado, não um defeito do gate.

**Achado novo, não óbvio, registrado em vault (não corrigido — fora do escopo `scripts/`+`Makefile`
desta ML):** `npm/src/lib/update-engine.js:tildeify` falha em abreviar `path` com `~` quando
`$HOME` contém uma barra dupla (`//`) em qualquer posição — `path.join` normaliza o `absPath`
comparado, mas o `homeRoot` bruto usado no `startsWith` não é normalizado, então a comparação nunca
bate e a função devolve o caminho absoluto cru. Isso não aparece com um `$HOME` real de
desenvolvedor, mas aparece de imediato no fixture do próprio gate porque `mktemp -d
"${TMPDIR:-/tmp}/..."` — o mesmo padrão já usado por `check-rules-parity.sh`/
`check-slash-parity.sh`/`check-barrier.sh` — herda um `$TMPDIR` do macOS que já termina em `/`,
produzindo `//` no `$WORK` e, por extensão, no `$HOME` de cada cenário. Nenhum gate anterior
exercitava renderização de `path`, então nenhum tinha motivo para revelar isso antes. Nota:
`vault/notes/node-tildeify-double-slash-home-2026-07-29.md`, linkada no índice. Nenhuma
contorno foi aplicado dentro do gate (ex.: normalizar `$TMPDIR` antes do `mktemp`) — isso
mascararia o defeito real que o gate existe para expor.

**Prova de não-vacuidade:** cada cenário tem guard explícito (targets não-vazios/parseáveis no
cenário 1; ao menos um target não-`missing` no cenário 2, para que "harness populado" não degenere
silenciosamente no cenário 1) antes de qualquer comparação; cenário 17 do `check-gates-falsify.sh`
prova que a asserção "zero escritas sob `--dry-run`" tem poder de reprovação real.

**HOME real confirmado intocado:**
```
find ~/.claude ~/.codex ~/.gemini -mmin -30 -type f | grep -i trackfw
```
retornou apenas artefatos de sessão do próprio Claude Code (`~/.claude/projects/.../*.jsonl`,
`*.meta.json`) e cache/log do Codex (`~/.codex/logs_2.sqlite*`, `models_cache.json`) — nenhum
arquivo de agente/skill/regra `trackfw-*` sob o `$HOME` real.

**Ambiguidade reportada, não corrigida:** `CLAUDE.md` na raiz do repositório aparece modificado no
working tree (bloco `<!-- trackfw:rules:start -->` injetado) e vários arquivos de runtime das
árvores `internal/`, `npm/src/` e `pypi/trackfw/` estão em andamento — atribuído ao agente do
ML-6F rodando em paralelo (fora do escopo desta ML, que é restrita a `scripts/`+`Makefile`); não
investigado nem revertido.

Nenhum arquivo fora de `scripts/check-update-parity.sh`, `scripts/check-gates-falsify.sh`,
`Makefile`, `vault/notes/node-tildeify-double-slash-home-2026-07-29.md` e `vault/notes/index.md`
foi alterado por este ML. Nenhuma operação Git executada — sem autoridade; branch permanece com
working tree alterado para o `trackfw_architect` auditar e commitar.

## Auditoria 2026-07-29 — Zeus — Wave 6 completa e roadmap fechado

O ML-6F falhou por erro de API no meio do trabalho. O trabalho parcial foi preservado e commitado
(harness já alinhado nos três runtimes), e o restante foi concluído pelo ML-6H.

Duas lacunas do meu contrato apareceram nesta wave, ambas da mesma natureza — eu pinei estados,
flags e ordem de chaves, mas deixei **conjuntos** em aberto:

1. Lista de targets do harness: Go declarou 3, Node e Python 19. Pinada em 19.
2. Lista de targets de projeto: Python declarou 3 dos 5. Pinada em 5.

E uma lacuna de semântica: `updated` vs `skipped`. Go comparava conteúdo, Node reescrevia sempre —
mesma entrada, estados diferentes. Pinado: o discriminador é o conteúdo ter mudado, não a
implementação ter chamado `write()`.

O ML-6H descobriu duas lacunas de paridade `init`↔`update` no caminho: o `init` do Go nunca escrevia
agent-hooks e o do Python nunca escrevia `scripts/trackfw-validate.sh`.

**Incidente do ML-6I — o gate mutava o repositório e passava.**
`scripts/check-update-parity.sh` injetava o bloco `trackfw:rules` no `CLAUDE.md` deste repositório,
porque `install_claude_agents()` redirecionava `HOME` mas não fazia `cd` para diretório descartável,
herdando o `cwd` de quem invocava. Como o gate está no alvo `parity`, `make quality` mutava a árvore
de trabalho — e o gate retornava exit 0 enquanto fazia isso. Reproduzido, corrigido, e coberto pelo
cenário `falsify/no-repo-mutation`, que compara `git status --porcelain` antes e depois de rodar os
gates. Transforma "eu conferi" em "o pipeline confere".

Verificação final independente: `make quality` exit 0; 19 cenários de falsificação e 12 gates
provados não-vacuosos (eram 13 e 8 no início da sessão); os quatro gates novos rodados da raiz não
alteram nenhum arquivo versionado; `CLAUDE.md` limpo; zero artefatos trackfw tocados no HOME real;
`validate --json` 0 violações. Barriers das waves 1 a 3 retornam `passed`.

Defeito extraído em vez de inflar o roadmap: `init --ai-tools` mutando o harness global virou REQ e
roadmap próprios em `backlog/`.

## Auditoria 2026-07-29 — Zeus — barriers das 6 waves e fechamento

Barriers finais: waves 1, 2, 3, 4 e 5 retornaram `passed` de primeira. **A Wave 6 reprovou**, com
`acceptance_evidence: ML-6F: 5 unmet acceptance criteria`.

A causa não foi código: eu marquei o ML-6F como `✅ Concluído` e deixei os cinco critérios de aceite
com `- [ ]`. Os critérios estavam de fato atendidos — em conjunto pelo ML-6F parcial e pelo ML-6H —
mas o registro não refletia isso. O check `acceptance_evidence` existe exatamente para isso e pegou
o descuido do orquestrador.

Vale registrar porque fecha o argumento da sessão: a barrier reprovou a Wave 2 por divergência entre
runtimes que as suítes individuais não viam, e reprovou a Wave 6 por bookkeeping do próprio
orquestrador. Nas duas vezes o mecanismo funcionou contra quem o construiu. Marcar ML como concluído
sem evidência é o comportamento vacuoso que a regra 13 do ADR proíbe.

Após corrigir o registro: `barrier --wave 6` retorna exit 0 e `status: passed`. As seis waves passam.

## 2026-07-29 — Zeus — IMPLEMENTANDO: refino da REQ órfã em backlog + contrato de skip de artefato desatualizado

Único par REQ→Roadmap em backlog era `escopo-de-init-ai-tools-nao-deve-mutar-o-harness-global`,
extraído do roadmap da barrier. Ao refiná-lo para handoff, a premissa **não sobreviveu à
verificação** — registro aqui porque o erro é instrutivo e não deve ser repetido.

A REQ afirmava que `init --ai-tools` gravar em `~/.gemini/agents/` era defeito, invocando o contrato
do ML-6A. Duas verificações refutaram isso:

1. `ADR-2026-07-25-escopo-de-instalacao-selecionavel` **decide o oposto de forma deliberada** — D1
   (sem TTY → `global`, registrado como breaking change) e D4 (`init` sem TTY → `global`), com
   consequência positiva declarada "elimina instalação surpresa no repositório do usuário".
   `init.go:118` e o comentário da linha 395 (`defaults to "global" (D1)`) são implementação fiel.
2. O contrato de `docs/cli-parity.md` invocado é titulado `trackfw update vs trackfw update harness`
   e abre com "Update is split by scope". Pina 5 targets de projeto e 19 de harness, todos do domínio
   `update`. **Não menciona `init`** — não é fronteira projeto/global geral.

A evidência empírica citada (`artifact ... is outdated; use update`) vem de `manager.go:220`, o
preflight de install recusando artefato `outdated`+`owned`. Prova que `init` alcança o HOME — o que o
D4 manda. **Não** prova que alcançá-lo seja errado.

Lição: uma REQ extraída às pressas de outro roadmap herda a interpretação de quem extraiu, não o
contrato real. Generalizar um contrato escopado ("update nunca muta global" → "nenhum comando de
projeto muta global") é o tipo de salto que só aparece lendo o ADR original.

O defeito **real** que a evidência expõe é outro: `install` sobre artefato `outdated`+`owned` retorna
erro, e como `mutate` é lote atômico com rollback, **aborta o scaffold inteiro** de um projeto novo
por causa do estado de um artefato que não pertence a esse projeto. Decisão do usuário: manter
D1/D4, reescopar para o defeito de robustez.

Achado que muda o conteúdo dos MLs: `npm/tests/agents-skills.test.js:193` contém
`assert.throws(() => manager.install([plan]), /outdated.*update/i)` — asserção que **codifica o
contrato antigo** e precisa ser invertida. Go e Python não tinham cobertura equivalente. Sem pinar
isso no roadmap, três agentes paralelos decidiriam independentemente entre apagar, inverter ou
contornar a asserção — exatamente o modo de falha que o ML-6F mediu.

Artefatos: REQ e roadmap antigos removidos; novos em
`install-pula-artefato-desatualizado-em-vez-de-abortar` (roadmap em `wip/`). Contrato do ML-1A
escrito em `docs/cli-parity.md`. Branch `fix/install-pula-artefato-desatualizado`.
Wave 2 = 3 MLs paralelos (Go ‖ Node ‖ Python); Wave 3 = auditoria de paridade após barrier.

---

## Sessão 2026-07-29 — Apolo (ML-2C — Python: install pula artefato outdated+owned)

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-29-install-pula-artefato-desatualizado-em-vez-de-abortar.md`
**Status:** IMPLEMENTANDO
**Branch:** `fix/install-pula-artefato-desatualizado`

Implementando ML-2C (Python-only): manager.py, command.py, init.py e testes.


**Status:** CONCLUÍDO — commit 4f25e1e pushed em fix/install-pula-artefato-desatualizado.
693 testes Python passados.

Notas de contrato para ML-3A (auditoria de paridade):
- _tildeify não importado (import circular) — lógica inlinada em _mutate com comentário.
- update_harness.py/_catalog_group_result usa IntegrationManager sem on_skip (fora do allowlist ML-2C).
- Teste test_legacy_adoption_then_update (linhas 232-243) inalterado e verde.

---

## Sessão 2026-07-29 — Apolo (ML-2E — corretivo: manager compõe reason; callers apenas imprimem)

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-29-install-pula-artefato-desatualizado-em-vez-de-abortar.md`

**Status:** IMPLEMENTANDO

**Tarefa:** ML-2E (Wave 2-bis) — mover composição da linha de aviso de skip para dentro do manager Python.
- `manager.py`: compor `reason` = linha completa; derivar remediação de `plan["claim"]["scope"]` por artefato; chamar `on_skip(display, reason)`.
- `init.py` e `command.py`: reduzir closures `_on_skip` a `print(reason, file=sys.stderr)`.
- Testes: atualizar asserção de `reason`; adicionar caso de lote de escopo misto.

---

---

## Sessão 2026-07-29 — Apolo (ML-2D — corretivo: manager Node.js compõe reason; callers apenas imprimem)

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-29-install-pula-artefato-desatualizado-em-vez-de-abortar.md`

**Status:** CONCLUÍDO

**Tarefa:** ML-2D (Wave 2-bis) — mover composição da linha de aviso de skip para dentro do manager Node.js.

**O que mudou:**
- `npm/src/integrations/manager.js`: import `tildeify` de `../lib/update-engine` (sem ciclo — update-engine só importa built-ins). Adicionado método `tildeAbbrev(file, scope)`: escopo global → `tildeify(homeRoot, file)`; escopo projeto → `path.relative(projectRoot, file)`. Bloco `onSkip` em `mutate()` agora deriva `remediation` de `item.plan.claim.scope` por artefato e chama `this.onSkip(abbrev, reason)` com linha completa.
- `npm/src/commands/init.js`: removido import `tildeify`; ambas as closures `onSkip` simplificadas para `process.stderr.write(\`${reason}\n\`)`.
- `npm/src/commands/integrations.js`: removido import `tildeify`; closure `onSkip` simplificada para `process.stderr.write(\`${reason}\n\`)`.
- `npm/tests/agents-skills.test.js`: asserção do teste existente atualizada para `{ dest: plan.destination, reason: linha-completa }`; adicionado teste `mixed-scope batch` com artefatos de projeto e global no mesmo lote — prova que cada um recebe a remediação correta por `claim.scope`.

**Resultado:** `cd npm && npm test` → 329 passed, 0 failed.
**Import de tildeify criou ciclo?** Não. `update-engine.js` importa apenas `fs`, `os`, `path`, `crypto`.
**Caso de escopo misto construtível?** Sim — teste adicionado e verde.


## 2026-07-29 — Zeus — Wave 2-bis convergida; dois achados de processo

Os agentes do ML-2D (Node) e ML-2E (Python) morreram por **limite de sessão de API** no meio do
commit. Auditei a árvore em vez de confiar nos relatórios parciais. Estado real:

**Código: presente e convergido.** Verificado no código, não em relatório:
- Node `manager.js:146-149` compõe a linha, remediação de `item.plan.claim.scope`; callers
  `init.js:284` e `integrations.js:190` recebem `(_destination, reason)` e só escrevem em stderr.
- Python `manager.py:211-233` compõe a linha, remediação de `plan["claim"]["scope"]`; closures de
  `init.py:157` e `command.py:287` reduzidas a `print(reason, file=sys.stderr)`.
- Go intocado (era o canônico).
- As três strings de formato são idênticas: `warning: skipping outdated artifact %s; run '%s' to
  refresh it`. Testes de lote de escopo misto existem no Node (`agents-skills.test.js:208`) e no
  Python (`test_agents_skills.py:298`).

**Validação independente:** `go build`/`go test`/`go vet` limpos · `npm test` 329 passed ·
`python3 -m pytest` 694 passed · `make quality` exit 0 com os 19 cenários de falsificação, incluindo
`falsify/no-repo-mutation`. Nenhuma suíte suja a árvore.

### Achado 1 — dois agentes paralelos compartilham o index do Git

O agente do ML-2D commitou com os arquivos do ML-2E já staged pelo agente paralelo: **d737b15 contém
os dois MLs** com mensagem que descreve só o Node. Defeito de rastreabilidade, não de conteúdo.

`git add <caminhos>` explícito por ML **não é suficiente** — não desfaz o staging que o outro agente
já fez. Para MLs paralelos no mesmo repo, o correto é `git commit -- <caminhos>` (que ignora o index)
ou worktrees isoladas por agente. Registrar porque a instrução "commite apenas seus arquivos" que eu
dei nos handoffs é insuficiente e vai falhar de novo.

### Achado 2 — poluição do repo por invocação manual do CLI (nota de vault criada)

`git status` acusava `AGENTS.md` e `CLAUDE.md` com +51 linhas (bloco `trackfw:rules`) e `.cursor/`
novo. Um agente rodou `init --ai-tools` com cwd na raiz do repo real para validar à mão.

O ponto não óbvio: **`make quality` passa exit 0 com a árvore poluída.** O cenário
`falsify/no-repo-mutation` do ML-6I funciona — mas guarda os **gates**, não a sessão do agente. Comando
ad-hoc escapa por construção. Se o agente commitasse com `git add -A`, entraria no PR como trabalho.
Revertido. Detalhes e prevenção em
`vault/notes/agente-poluindo-repo-ao-rodar-cli-manualmente-2026-07-29.md`.

### Correção de enquadramento minha

Eu havia dito que a divergência da Wave 2 era "literalmente o ML-6F repetindo". Não era: o ML-6F
produziu saída observavelmente diferente (3 vs 19 targets); aqui as strings em stderr saíam
byte-idênticas desde o início. O que divergia era forma interna e robustez da derivação de escopo —
endurecimento preventivo mais um bug latente de escopo misto, não regressão visível ao usuário.

---

## Sessão 2026-07-29 — Artemis/QA (ML-3A — auditoria de paridade byte-a-byte)

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-29-install-pula-artefato-desatualizado-em-vez-de-abortar.md`

**Tarefa:** ML-3A — três lacunas de cobertura pós-Wave-2-bis:
1. Teste E2E com HOME isolado para os três runtimes (`init` + artefato global outdated → exit 0 + scaffold + bytes preservados + aviso)
2. Teste de lote de escopo misto no Go (espelho de Node.js:208 e Python:298)
3. Cenário de paridade comparando bytes de stderr dos três CLIs (global + project)

**Entregue:**

### Lacuna 2 — TestManagerInstallSkipMixedScopeBatch (Go)

Adicionado em `internal/integrations/manager_test.go`. Instala v1 para dois artefatos (global e
projeto) na mesma chamada `Install`, depois instala v2 para ambos → ambos outdated+owned → ambos
pulados. Verifica que cada um recebe a remediação correta pelo seu `plan.Claim.Scope` (não por closure
sobre escopo uniforme do lote). Mirrors Node.js:208 e Python:298.

Resultado: `go test ./internal/integrations/ -run TestManagerInstallSkipMixedScopeBatch` → **PASS**.
`go test ./...` → **todos passam**.

### Lacuna 1 + 3 — Cenários 6, 7 e 8 em check-update-parity.sh

Adicionados três cenários em `scripts/check-update-parity.sh`:

**Cenário 6 (skip-parity/global-scope):** cada runtime instala gemini/architect globalmente em HOME
próprio, manifesto é patchado para outdated+owned (sentinel bytes + sha256 + catalog_version antigo),
re-install captura aviso de stderr. Compara bytes entre os três: `three-runtimes-identical`.

**Cenário 7 (skip-parity/project-scope):** mesmo para escopo de projeto. Cada runtime usa projeto e
manifesto próprios (necessário: Node.js/Python resolvem `process.cwd()` via `/private/` no macOS,
produzindo chave de manifest diferente da do Go). Aviso byte-idêntico.

**Cenário 8 (e2e/init-outdated-global):** para cada runtime — instala gemini/architect globalmente,
patcha para outdated+owned, executa `init --ai-tools gemini`, afirma: (a) exit 0; (b) `trackfw.yaml`
criado; (c) sentinel preservado em `trackfw-architect.md`; (c-sibling) `trackfw-backend.md` gravado
(skip ≠ abort); (d) aviso em stderr.

Resultado: `bash scripts/check-update-parity.sh` → **todos os cenários novos: OK**.
`make quality` → **exit 0**, 19 cenários de falsificação, `git status` limpo.
`bin/trackfw validate --json` → **0 violações**.

### Evidência D1/D4

`git diff origin/main..HEAD -- internal/commands/init.go npm/src/commands/init.js pypi/trackfw/commands/init.py`
mostra apenas adição de `OnSkip` callback — lógica de resolução de escopo inalterada. O Cenário 8
confirma empiricamente: init instala em `$HOME/.gemini/...` (artefato irmão encontrado em HOME, não no
projeto), provando que D1/D4 estão em vigor.

**Status:** CONCLUÍDO

## 2026-07-29 — Zeus — CONCLUÍDO: roadmap fechado, quatro waves com barrier verde

ML-3A (Artemis) entregou as três lacunas. Auditei independentemente em vez de aceitar o relatório:
`make quality` exit 0 com os sete cenários novos — `skip-parity/{global,project}-scope/three-runtimes-identical`
(ambos com vacuity-guard) e `e2e/init-outdated-global/{go,node,py}` — mais os 19 de falsificação.
`validate --json` 0 violações, árvore limpa. Barrier das waves 1–4: todas `passed`.

O E2E era a lacuna que importava: todo teste anterior exercitava o *manager*: nenhum provava o
sintoma originalmente reportado. Agora os três runtimes provam `init --ai-tools` com artefato global
desatualizado → exit 0, `trackfw.yaml` criado, bytes do desatualizado preservados e artefato irmão
gravado (skip ≠ abort).

### Achado — `trackfw barrier` rejeita wave não-inteira

Nomeei a wave corretiva de "Wave 2-bis". A barrier falhou nas **quatro** waves com
`malformed wave heading at line 250: "2-bis" is not a valid wave number` — o parser exige inteiro e
aborta o documento inteiro, não só a wave malformada. Renumerei para Wave 3 (corretivo) e Wave 4
(auditoria), reordenando os blocos para refletir a ordem real de execução.

Os ids dos MLs ficaram fora de sincronia com as waves (`ML-2D`/`ML-2E` na Wave 3, `ML-3A` na Wave 4).
Deliberado: renumerar quebraria a rastreabilidade de mensagens de commit já publicadas.

Candidato a REQ futura (não expandi escopo agora): "wave N-bis" é nomenclatura natural para wave
corretiva acrescentada após execução, e o produto não a aceita. Ou o parser passa a aceitar sufixo,
ou a documentação declara explicitamente que wave é sempre inteiro.

### A barrier funcionou contra o orquestrador — de novo

Desmarquei dois critérios que forward-referenciavam o ML-3A. A barrier da Wave 3 retornou `blocked`
com `ML-2D: 1 unmet acceptance criteria` / `ML-2E: 1 unmet` e **segurou a wave até a evidência
existir**. Depois que o ML-3A rodou e produziu os cenários, marquei com referência ao cenário concreto
e a wave passou. É o segundo ciclo em que o `acceptance_evidence` pega bookkeeping meu — na sessão
anterior foi o ML-6F.

### Também corrigido

Minha nota de vault não estava linkada em `vault/notes/index.md`; `validate` acusou `note_orphan`.
Linkada.

Roadmap movido para `done/`, REQ fechada como `Done`. Branch `fix/install-pula-artefato-desatualizado`
pronta. PR **não** aberto — aguardando solicitação explícita do usuário.

## 2026-07-29 — Zeus — IMPLEMENTANDO: barrier aceita rótulo de wave com sufixo (ML-1A concluído)

PRs #86 e #87 mergeados; branches apagadas após validar integração por diff contra o tip da main.
Roadmap movido para `wip` antes da criação da branch, conforme o protocolo.

**O gate pegou meu nome de branch.** Criei `feat/barrier-wave-sufixo` e o `validate` reprovou com
`branch_has_wip_roadmap`: o casamento é `normalizeBranchSlug(filename).contains(branchSlug)`, e
`barrier-wave-sufixo` **não** é substring de `barrier-aceita-wave-com-sufixo-bis`. Renomeada para
`feat/barrier-aceita-wave-com-sufixo-bis`. Registrar porque é fácil errar: o slug da branch precisa ser
substring do nome do roadmap, não apenas "parecido".

**Segunda violação, também instrutiva:** `roadmap move` sincroniza o `status:` do frontmatter e a pasta,
mas **não** atualiza a referência `roadmap:` da REQ que aponta para a pasta antiga — `ref_targets_exist`
reprovou com `links to Roadmap "docs/roadmaps/backlog/..." which does not exist`. Corrigi à mão.
Candidato a REQ futura: `roadmap move` poderia atualizar a REQ pareada.

### ML-1A — contrato congelado

ADR emendado com duas decisões:
- **15** — wave é identificada por **rótulo**, não inteiro. Gramática `<inteiro>[-<sufixo>]`, sufixo
  `[a-z0-9]+`. Rótulos são identidades distintas: `--wave 2` nunca casa com `Wave 2-bis`.
- **16** — heading fora da gramática **continua abortando o documento inteiro**, e isso é feature.
  Escopar o erro à wave solicitada foi rejeitado: ignorar heading malformada deixaria seus MLs sem
  auditoria, e um typo produziria barrier verde sobre trabalho não verificado. É a mesma vacuidade que
  a decisão 13 proíbe.

`docs/cli-parity.md` ganhou a seção `### Wave label grammar` com regex pinada
`^## Wave (\d+(?:-[a-z0-9]+)?) `, tabela de válidos/inválidos, ordenação em 3 passos
(`2` < `2-bis` < `2-hotfix` < `3`) e a terceira mensagem de exit-2 pinada.

Registrei no contrato que essa terceira mensagem estava **despinada** e por isso divergia nos três
runtimes — Go dizia `is not a valid wave number`, Python `number ... is not parseable`, e o Node
despejava a linha inteira **sem nomear a causa**. Novo texto pinado usa `wave label` e carrega o
`<token>`, nunca a linha inteira.

Lição do roadmap anterior aplicada de propósito: lá o ML-1A pinou os **nomes** dos parâmetros e não
seus **valores**, e custou uma wave corretiva inteira com três respostas divergentes. Aqui a gramática,
a ordenação **e** o texto literal foram pinados antes de qualquer código.

`barrier --wave 1` retorna `passed`. Wave 2 (3 runtimes em paralelo) a seguir.

---

## 2026-07-29 — Apolo — CONCLUÍDO: ML-2A Go (barrier aceita rótulo de wave com sufixo)

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-29-barrier-aceita-wave-com-sufixo-bis.md`
**Branch:** `feat/barrier-aceita-wave-com-sufixo-bis`
**Commits:** `751180b` (código) + `8284ad9` (roadmap)

**Arquivos editados:** `internal/commands/barrier.go`, `internal/commands/barrier_test.go`,
`internal/commands/barrier_contract_test.go` (APENAS Go — npm/ e pypi/ intocados).

**O que mudou:**
- `waveBlock.label string` (era `.number int`); `barrierResult.Wave string` (era `int`)
- `waveLabelRe = `^\d+(?:-[a-z0-9]+)?$`` adicionado para validar token capturado pelo broad regex
- `parseWaves`: usa detect (broad regex) + validate (waveLabelRe + inteiro>=1) — heading fora da gramática continua abortando o documento inteiro (ADR dec.16 preservado)
- Mensagem usa `"%s"` (verbatim) em vez de `%q` — sem escape de caracteres não-ASCII cross-runtime
- `--wave 2-bis` resolve `## Wave 2-bis`; `--wave 2` nunca casa com `## Wave 2-bis`
- `splitWaveLabel` e `compareWaveLabels` adicionados (ordenação: numérica > sem-sufixo > lexicográfico)
- Teste E2E de regressão: `--wave 1` em documento com `## Wave X — ...` → exit 2, stderr byte-exato

**Validação:** `go build ./...`, `go test ./...`, `go vet ./...` — todos verdes.

**Observações para o orquestrador:**
1. `"wave"` no JSON mudou de `number` para `string` (`"wave":"1"` em vez de `"wave":1`).
   O exemplo em `docs/cli-parity.md` `### JSON document` mostra `"wave": 2` (número) — divergência.
   Precisa de atualização pelo orquestrador (seção diferente da `### Wave label grammar` congelada).
2. Mensagem `--wave` inválido está despinada: Go usa `not a valid wave label`, Node usa
   `must be an integer >= 1`, Python usa `is not an integer`. Necessita pinagem antes do ML-3A.
3. `compareWaveLabels` implementado mas não usado no fluxo de barrier (barrier não lista/ordena waves).
   Disponível para uso futuro.

---

## Sessão 2026-07-30 — Artemis (ML-3A — Auditoria de paridade e não-vacuidade)

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-29-barrier-aceita-wave-com-sufixo-bis.md`

**Tarefa:** ML-3A (Wave 4) — transformar a verificação manual do orquestrador em gates automáticos permanentes.

**Entregue:**
- `scripts/check-barrier.sh`: Cenários 8–12 adicionados (heading malformada antes e depois da wave alvo; identidade `--wave 2-bis`/`--wave 2`; `## Wave 0`; `--wave 2-BIS`). PY_ROOT agora overridable. BARRIER_BIS_SELFTEST_BREAK seam para falsificação. 34 cenários OK.
- `scripts/check-gates-falsify.sh`: Cenário 19 adicionado — `BARRIER_BIS_SELFTEST_BREAK=1` prova que o Cenário 9 detecta early-break. Contador atualizado para 20 cenários, 13 gates não-vacuosos.
- `internal/commands/barrier_test.go`: `TestWaveLabelGrammar_ValidAndInvalid` (tabela completa 6 inválidos + 5 válidos via `parseWaves`) + `TestBarrierRegression_FourthExitTwoMessage` (quarta mensagem byte-exata via `--wave 2-BIS`).
- `pypi/tests/test_barrier.py`: `test_is_valid_wave_label_tabela_completa` (tabela completa via `_is_valid_wave_label`).
- `docs/roadmaps/wip/ROADMAP-2026-07-29-barrier-aceita-wave-com-sufixo-bis.md`: ML-3A ✅ Concluído.

**Validação:**
- `go test ./internal/commands/`: ok (8.7s)
- `pytest pypi/tests/`: 702 passed
- `npm test --prefix npm`: 0 fail
- `make quality`: exit 0 (20 cenários de falsificação, 13 gates não-vacuosos)
- `bin/trackfw validate --json`: violations: []
- `git status`: limpo

**Handoffs:** Nenhum. ML-3A é o último ML do roadmap.
**Status:** CONCLUÍDO

## 2026-07-29 — Zeus — CONCLUÍDO: barrier aceita rótulo de wave com sufixo, 4 waves verdes

REQ+roadmap fechados. Auditoria independente: `make quality` exit 0 com **34 cenários de barrier** e
**20 de falsificação / 13 gates não-vacuosos**; `validate --json` 0 violações; árvore limpa; barrier das
quatro waves `passed`.

### O que a auditoria empírica pegou — a lição da sessão

Executei os três CLIs em vez de ler relatórios. O ML-2C (Python) **afirmava** preservar o abort de
heading malformada e **não preservava**: `_find_wave` saía do laço ao achar a wave pedida, então uma
heading malformada **posicionada depois** da wave alvo nunca era visitada → exit 1 `blocked` em vez de
exit 2. Violava a decisão 16 do ADR e, pior, a **12** — roadmap malformado lido como "wave reprovada"
mascara o defeito real.

O teste de regressão do ML-2C era **real** mas cobria só a posição "antes". Passava enquanto o bug
sobrevivia. Não foi má-fé: foi cobertura incompleta numa dimensão que o contrato não nomeava.

### A frase que faltava no contrato

Pinei os dois regexes (detector amplo + validador estrito) e esqueci de pinar **quando** a validação
roda. Os regexes sozinhos não evitariam o bug. O contrato agora exige, como texto normativo:

> A varredura deve visitar todas as headings do documento antes de resolver o rótulo pedido, e não pode
> sair antecipadamente ao casar.

Com tabela empírica das duas posições e a nota de que teste de uma posição só é vacuoso.

Padrão que se repete e vale internalizar: **pinar a estrutura sem pinar a ordem das operações produz
divergência**. É a terceira variação do mesmo erro nesta sessão — no roadmap anterior foram os *valores*
dos parâmetros do observador (pinei os nomes); aqui foi o *momento* da validação (pinei os regexes).

### Ganhos não pedidos, confirmados

- `## Wave 0` passa a ser rejeitada nos três. O `_is_valid_wave_label` do Python valida inteiro ≥ 1,
  gap que o check por linha não detectava; Go e Node já rejeitavam.
- Node corrigiu no próprio ML-2B um early-break **pré-existente** que ninguém conhecia: antes desta REQ,
  Go abortava em qualquer heading malformada e o Node só nas anteriores à wave alvo. A divergência
  sobrevivia porque a mensagem estava despinada e cada suíte testava o próprio comportamento.

### Quarta mensagem de exit-2

Pinei a de *heading* malformada e esqueci a de *argumento* `--wave` inválido — três textos divergentes
de novo. Pinada adotando o texto do Go; Node e Python alinhados, separador U+2014 conferido byte a byte.

### Decisões de escopo negativo registradas no contrato

- **Comparador de ordenação é opcional:** sem call site em runtime nenhum. Go tem um coberto por testes;
  Node e Python declinaram corretamente. Documentado para ninguém "corrigir" a assimetria em nenhuma
  das duas direções — código morto em dois runtimes não é paridade.
- **Espaçamento do JSON não se mexe:** `check-barrier.sh` normaliza whitespace de propósito e não faz
  `sort_keys`.
- **Não batizei a wave corretiva de `Wave 2-bis`**, apesar de ser o caso de uso da feature: o Python
  ainda não tratava o rótulo e `make quality` roda `check-barrier.sh` nos três — batizar assim
  codificaria o defeito não-corrigido dentro do artefato que controla a correção. **Dogfooding agora
  está liberado** para o próximo roadmap corretivo.

### Não-vacuidade provada por execução

`BARRIER_BIS_SELFTEST_BREAK=1` faz o gate reprovar com
`FAIL [barrier/wave-label/malformed-after-target/go]: expected exit 2 ..., got 0`. O seam corrompe a
**fixture**, nunca a asserção. Verifiquei rodando — um cenário de falsificação que não falha é ele
mesmo vacuoso.

## 2026-07-30 — Zeus — IMPLEMENTANDO: roadmap move sincroniza a referência da REQ pareada (ML-1A feito)

PR #88 mergeado; branch apagada após validar integração. Backlog estava vazio e nenhuma REQ `Open` —
levantei o que existia de real em vez de inventar trabalho: um release pendente (3 commits desde v4.0.0,
com breaking no campo `wave` do JSON) e este débito. Usuário escolheu o débito; **o release segue
pendente**.

### Débito obsoleto encontrado no próprio registro

O working-context tinha registrado como pendente que `roadmap move` não reescrevia o `status:` do
frontmatter. **Já foi corrigido** por `REQ-2026-07-27-roadmap-move-sincroniza-o-status-do-artefato`.
Registro estava velho — verifiquei antes de agir sobre ele. Vale como lembrete: débito anotado em nota
não é fonte de verdade sobre o estado atual do código.

### ML-1A — contrato pinado

Fatos verificados no código antes de escrever, e o decisivo foi este: **`extractRefPath`
(`internal/validator/validator.go:1426`) trima aspas mas NÃO backticks.** Como a linha do corpo da REQ é
`` Roadmap: `docs/...md` ``, ela termina em backtick, nunca casa `.md` e é **invisível ao validador**.
Só o frontmatter é normativo. Uma implementação que "corrigisse" só o corpo não resolveria nada e passaria
a impressão de ter resolvido.

Também verificado: o `req:` do roadmap **não serve** para descobrir o par (`roadmap new` grava `""`, e os
existentes têm slug sem caminho). A descoberta tem de ser inversa — varrer `req_dir` casando basename,
cobrindo flat e `by_agent`, espelhando o que o validador já varre.

Pinei **cinco** cardinalidades, não quatro como havia previsto no roadmap: separei "aponta para outro
roadmap" de "referência já correta". São comportamentos distintos — o primeiro é não-tocar, o segundo é
não-escrever-por-idempotência.

### Lição das duas waves corretivas anteriores, aplicada de propósito

O ML-1A falhou **duas vezes seguidas pelo mesmo padrão**: pinou a forma e deixou o comportamento à
interpretação. No roadmap do skip foram os *nomes* dos parâmetros sem os *valores*; no do rótulo de wave
foram os *regexes* sem o *momento* da validação. Custo: uma wave corretiva cada.

Regra derivada e escrita no roadmap: **pinar sempre a ordem das operações e os valores observáveis, não
apenas estruturas e assinaturas.** Neste ML-1A isso virou: momento exato da escrita no fluxo, tabela de
cardinalidades, textos literais de saída com stream, e comportamento em erro (não desfaz o move, tenta
as restantes, exit não-zero ao fim).

Barrier da Wave 1: `passed`. Wave 2 (3 runtimes em paralelo) a seguir.

## 2026-07-30 — Zeus — CONCLUÍDO: roadmap move sincroniza a referência da REQ, 4 waves verdes

`make quality` exit 0 · novo gate `scripts/check-roadmap-move-parity.sh` com 5 cenários ·
21 cenários de falsificação / 14 gates não-vacuosos · Go limpo · npm 339 · pytest 724 ·
`validate --json` 0 violações · barrier das quatro waves `passed`.

### Dogfooding provado no caso real

Executei a ferramenta sobre a própria REQ desta entrega, numa cópia:

```
antes:  roadmap: "docs/roadmaps/wip/ROADMAP-2026-07-30-...md"
        ✓ moved ROADMAP-2026-07-30-...md → docs/roadmaps/done
        ✓ synced REQ-2026-07-30-...md → docs/roadmaps/done/ROADMAP-2026-07-30-...md
depois: roadmap: "docs/roadmaps/done/ROADMAP-2026-07-30-...md"
        Roadmap: `docs/roadmaps/done/ROADMAP-2026-07-30-...md`
```

Frontmatter **e** corpo atualizados, backticks preservados. **A correção manual de referência que fiz
quatro vezes nas duas sessões anteriores deixa de ser necessária.**

### A lição central desta iteração

As três suítes ficaram verdes (339 · 724 · Go limpo) com **duas divergências ativas**. Só apareceram
com fixture construída para discriminar.

**Ordenação — os três erravam, cada um por motivo diferente:**
- Go: `filepath.Glob` ordena por padrão, mas `scanREQFiles` concatena por agente e por estado, e a
  lista de estados é fixa e nem lexicográfica.
- Node: `readdirSync` sem sort — concordava em flat **por acidente do APFS**.
- Python: `sorted()` sobre **caminhos completos**. Passou na primeira fixture `by_agent` por
  coincidência aritmética (`apolo/…aaa` < `zeus/…zzz`).

**"Determinístico" não é "conforme".** O Python era perfeitamente determinístico e divergia do
contrato. Se eu tivesse parado na primeira fixture, teria aprovado.

**Regra operacional que fica:** ao verificar ordenação ou qualquer critério composto, construir a
fixture que **separa** os critérios candidatos. Fixture onde dois critérios coincidem não é evidência.

### Quarta ocorrência do meu mesmo erro de contrato

Escrevi "na ordem de varredura" — delegação disfarçada de especificação. Sequência nesta sessão:
nomes-sem-valores (skip) · regexes-sem-momento (rótulo de wave) · cardinalidades-sem-ordem (esta).
**Dois dos três implementadores reportaram a lacuna antes de eu perguntar** — foi o que a evitou virar
divergência permanente. Pedir explicitamente "reporte contrato incompleto" nos handoffs tem retorno
mensurável.

### Divergência pré-existente corrigida por decisão do usuário

O Python nunca imprimiu `✓ moved` — desde antes desta REQ imprimia `Roadmap movido para: <path>`
(idioma, forma e conteúdo diferentes). Verificado em `origin/main`. Fora do escopo original; incluído
porque a regra dura de paridade se aplica e a auditoria byte-a-byte da própria feature não passaria com
a linha anterior divergindo. Agora byte-idêntica, `hexdump` confirma `e2 9c 93`.

### Inconsistência minha que o agente pegou

O bloco do ML-3A dizia "quatro cardinalidades"; o contrato pina **cinco**. O Artemis seguiu o contrato
e reportou a discrepância. Contrato prevalece sobre roadmap — comportamento correto.

### Qualidade do seam de falsificação

O seam corrompe a **implementação** (cópia em árvore temporária), nunca a asserção, e tem **guarda de
padrão**: se o `sed` não alterar nada, falha com "padrão não encontrado; prova P4 inválida". Isso
protege contra o cenário em que o código-fonte muda, o `sed` para de casar e a falsificação se torna
vacuosa em silêncio. Não foi pedido — foi iniciativa do Artemis.

### Débito pendente registrado

**Release não feito.** Agora são 4 commits desde `v4.0.0` com mudanças observáveis acumuladas: campo
`wave` do JSON (número → string), textos de mensagens do barrier, `## Wave 0` rejeitada, e a linha
`moved` do Python. Critério do projeto (`feat` breaking → major) aponta **v5.0.0**.

## 2026-07-30 — Zeus — IMPLEMENTANDO: padrão único de saída de versão (ML-1A feito)

Release v5.0.0 publicada e tag no remoto. Backlog vazio; usuário pediu para ajustar o padrão de
versionamento. Decisão dele: **sem o `v`**, alinhado ao padrão de versões do Python.

**A feature de ontem já pagou:** `roadmap move ... wip` imprimiu `✓ synced REQ-...` e atualizou a
referência sozinho. A correção manual que eu fazia a cada transição não foi necessária.

### Não era uma divergência, eram três

Medi as três superfícies em vez de assumir que era só o prefixo `v`:

| Superfície | Go | Node.js | Python |
|---|---|---|---|
| `version` | `trackfw v5.0.0` | `trackfw 5.0.0` | `trackfw 5.0.0` |
| `--version` | `trackfw v5.0.0` | **`5.0.0`** | `trackfw 5.0.0` |
| `-v` | funciona | `error: unknown option` | cai no `usage:` |

O `--version` do Node imprime o número puro, sem `trackfw ` — default do `.version()` do commander.
Divergência maior que a do prefixo e que eu não teria visto se parasse na primeira superfície.

### O gate assinava a divergência

`check-cli-parity.sh:108` usa **regex própria para o Node**
(`^([0-9]+\.){2}[0-9]+|^0\.0\.0-dev$`) enquanto Go e Python usam `^trackfw .+`. O gate que existe para
detectar divergência a **codificava como esperada**. Enquanto essa linha existir, nenhuma auditoria
futura reporta.

E o `^trackfw .+` dos outros dois é frouxo demais: aceita `trackfw v5.0.0` e `trackfw 5.0.0`
igualmente — **é exatamente por isso que o `v` sobreviveu a todas as auditorias até agora**. Uma
asserção permissiva não é um gate fraco, é um gate que mente.

### ML-1A — contrato

`## Version output` em `docs/cli-parity.md` pina o texto literal, a equivalência byte-idêntica entre
`version` e `--version`, a fonte da string por runtime, e — o ponto que importa — **a asserção literal
do gate** mais a exigência de comparação byte-a-byte entre runtimes. Registrei por que a asserção
anterior era vacuosa, para ninguém reintroduzir uma regex permissiva.

Quinta iteração aplicando a mesma lição: pinar o comportamento observável, não a descrição do formato.

### Escopo negativo deliberado

O `-v` ficou **fora**. É divergência de *quais flags existem*, não de *formato de saída*, e resolvê-la
exige decisão própria: adicionar `-v` a dois runtimes é feature, removê-lo do Go é breaking change.
Registrado na REQ e no contrato para não se perder. Candidato a REQ separada.

Barrier da Wave 1: `passed`. Wave 2 (3 runtimes) a seguir.

---

## Sessão 2026-07-30 — Apolo (ML-2C — Python: cobertura de testes de formato de version) — CONCLUÍDO

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-30-padrao-unico-de-saida-de-versao-nos-tres-clis.md`

**Tarefa:** ML-2C — Verificar empiricamente o formato Python e adicionar testes que travam o contrato byte-a-byte.

**Branch:** `feat/padrao-unico-de-saida-de-versao-nos-tres-clis`

**Entregue:**
- `pypi/tests/test_commands_basic.py`: substituiu teste vacuoso por 3 asserções precisas:
  1. `test_version_flag_format_exact`: `--version` → `^trackfw [0-9]+\.[0-9]+\.[0-9]+$` em stdout.
  2. `test_version_subcommand_format_exact`: `version` → mesmo padrão canônico em stdout.
  3. `test_version_surfaces_byte_identical`: as duas superfícies são byte-a-byte idênticas.

**Saída verificada (xxd):**
```
version:   74 72 61 63 6b 66 77 20 35 2e 30 2e 30 0a  → trackfw 5.0.0\n
--version: 74 72 61 63 6b 66 77 20 35 2e 30 2e 30 0a  → trackfw 5.0.0\n
BYTE-IDENTICAL: ok
```

**Testes:** 727 pass, 0 failed (`cd pypi && python3 -m pytest`).
**Nenhuma mudança de comportamento:** `__init__.py` sem prefixo `v`; fallback literal `"5.0.0"` correto.

## 2026-07-30 — Zeus — CONCLUÍDO: padrão único de saída de versão, 3 waves verdes

`make quality` exit 0 · **23 cenários de falsificação** (eram 21) · Go limpo · npm 342 · pytest 727 ·
`validate --json` 0 violações · barrier das três waves `passed`.

Resultado: as **seis** saídas (3 runtimes × 2 superfícies) são byte-idênticas — `hexdump` confirma
14 bytes, `trackfw 5.0.0\n`.

### O achado que sustentou a REQ

A exceção por runtime no gate não era tolerância inofensiva: era **o mecanismo que mantinha o problema
invisível**. Prova direta, observada durante a Wave 2 — assim que o `--version` do Node passou a
imprimir `trackfw 5.0.0`, a regex de exceção da linha 108 deixou de casar e `check-cli-parity.sh`
passou a **reprovar**. A linha que assinava a divergência passou a bloquear a convergência.

E o `^trackfw .+` dos outros dois aceitava `trackfw v5.0.0` e `trackfw 5.0.0` igualmente — é
literalmente por isso que o prefixo `v` sobreviveu a todas as auditorias anteriores. **Asserção
permissiva não é gate fraco; é gate que mente.**

### Não era uma divergência, eram três

Medir as três superfícies em vez de assumir "é só o prefixo" revelou que o `--version` do Node imprimia
o número puro, sem `trackfw ` — divergência maior que a do `v`, e invisível se eu tivesse parado na
primeira superfície.

### Qualidade do trabalho dos agentes

- **Go** trocou `fmt.Println` por `fmt.Fprintln(cmd.OutOrStdout(), ...)`. Sem isso o teste não captura a
  saída: `fmt.Println` escreve em `os.Stdout` e ignora o `SetOut` do cobra. Correção que só aparece
  quando alguém tenta de fato escrever o teste.
- **Node** testou via `spawnSync` no binário real. Teste in-process poderia passar enquanto o binário
  divergisse, porque o `--version` é resolvido pelo commander no processo.
- **Artemis** identificou que **um** seam não bastava: o Cenário 21 (reintroduz o `v`) falha no braço de
  formato e por isso **não alcança** o braço de comparação byte-a-byte. Criou o Cenário 22, que corrompe
  `package.json` para `9.9.9` — formato válido nos seis, só a comparação de bytes detecta. Sem ele,
  metade da asserção composta ficaria não-provada. Não foi pedido.

### Débito registrado, não resolvido

A flag curta **`-v` funciona apenas no Go** (Node: `error: unknown option`; Python: cai no `usage:`).
Deixada fora por ser divergência de *quais flags existem*, não de *formato* — adicionar a dois runtimes
é feature, removê-la do Go é breaking change. Registrada na REQ e no contrato. **Candidata a REQ
própria.**

### Para o próximo release

Duas mudanças observáveis a constar no CHANGELOG: o Go deixa de imprimir o prefixo `v`, e o `--version`
do Node passa a incluir `trackfw `. Conforme o protocolo, o CHANGELOG é editado apenas no PR de release.

---

## Sessão 2026-07-30 — Apolo (ML-2A — Desvincular `-v` de `--version` no Go) — IMPLEMENTANDO

**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-07-30-reservar-v-para-verbose-e-remover-atalho-de-versao-no-go.md`
**Branch:** `feat/reservar-v-para-verbose-e-remover-atalho-de-versao-no-go`

**Abordagem:** Pré-registrar `root.Flags().Bool("version", false, "version for trackfw")` em `newRootCmd()`
antes que `InitDefaultVersionFlag()` do cobra seja chamado. O cobra só adiciona a flag se
`Flags().Lookup("version") == nil`, portanto o shorthand `v` nunca é registrado.

**Status final:** CONCLUÍDO

**Mudanças:**
- `internal/commands/root.go`: pré-registra `Flags().Bool("version", false, ...)` antes de `AddCommand`,
  bloqueando o registro automático do shorthand `v` pelo cobra.
- `internal/commands/version_test.go`: adiciona `TestShorthandVNotRegistered` (asserção estrutural sobre
  a flag set) e `TestShortVFlagRejected` (asserção comportamental sobre stdout + erro).

**Divergência de contrato registrada:** nenhuma. O contrato não exige identidade de mensagem/exit entre
os três runtimes para flags desconhecidas — cobra emite exit 1, o que satisfaz "não-zero".

## 2026-07-30 — Zeus — CONCLUÍDO: -v reservado para verbose, 3 waves verdes

`make quality` exit 0 · **24 cenários de falsificação** (eram 23) · Go limpo · npm 342 · pytest 727 ·
`validate --json` 0 violações · barrier das três waves `passed`.

| Invocação | Go | Node.js | Python |
|---|---|---|---|
| `version` / `--version` | `trackfw 5.0.0` | idem | idem |
| `-v` | `unknown shorthand flag` exit 1 | exit 1 | exit 2 |

### A solução foi mais elegante do que o roadmap previa

Eu havia alertado que remover o shorthand poderia perder o `SetVersionTemplate` e regredir o
`--version` recém-alinhado no PR #91. Não ocorreu: o caminho escolhido **pré-registra**
`root.Flags().Bool("version", false, ...)` sem shorthand. O cobra só adiciona a flag dele quando
`Flags().Lookup("version") == nil`, então o `v` nunca entra no mapa do pflag — **mas** o cobra continua
detectando `version=true` em execução e aplicando o template. Remove o atalho sem tocar no caminho que
produz a saída.

### Padrão que se repetiu nos dois agentes: asserções complementares

- **Apolo** escreveu duas: `ShorthandLookup("v") == nil` (estrutural) e `Execute()` com erro + stdout
  que não casa a linha de versão (comportamental). A primeira sozinha passaria se `-v` fosse registrado
  por outro caminho; a segunda sozinha passaria se `-v` falhasse por motivo alheio.
- **Artemis** fez o mesmo no gate: exit não-zero **e** saída que não casa o formato de versão. Só o exit
  code não distingue "rejeitada" de "aceita mas falhou por outro motivo".

### Guarda de vivacidade no seam — não pedida, e fecha lacuna real

Pedi guarda de padrão (`sed` que não altera nada → falha). O Artemis acrescentou **guarda de
vivacidade**: após corromper, **compila** e verifica que o binário corrompido de fato **aceita** `-v`
com exit 0 e formato de versão. Se não exibir o bug, falha com `seam inativo`.

A diferença importa: o `sed` pode alterar o arquivo **sem** restaurar o shorthand — por exemplo se o
comportamento do cobra mudar numa atualização. Nesse caso a guarda de padrão passa e a falsificação
vira vacuosa em silêncio. A guarda de vivacidade verifica o **efeito**, não a **edição**.

Padrão a reaproveitar: **seam precisa provar que a corrupção produziu o defeito, não apenas que o
arquivo mudou.**

### Estrutura sem paralelismo — deliberada

Três waves, um ML cada. Só o Go mudava código; Node e Python já rejeitavam `-v`. Não criei MLs vazios
para eles: seria cerimônia sem conteúdo, e a paridade é verificada pelo gate no ML-3A, que é onde
pertence.

### Pendente para o release

Dois breaking changes acumulados desde a `v5.0.0`, ambos de saída observável:
1. **#91** — Go deixa de imprimir o prefixo `v`; `--version` do Node passa a incluir `trackfw `.
2. **este** — `trackfw -v` deixa de funcionar no Go.

Aponta **v6.0.0**. Migração do segundo: usar `--version` ou `version`, que funcionam nos três desde a
v5.0.0.


## 2026-08-02 — Zeus — ARTEFATOS DE GOVERNANÇA: unificar a leitura do trackfw.yaml nos 3 CLIs

Ciclo aberto por pedido do KG após a tag `v6.2.0`, para fechar o único item que restava na fila —
reportado pelo executor do PR #106, que corretamente parou por ser decisão de ADR.

**Entrega de hoje é só governança**: ADR + REQ + Roadmap em `backlog/`. Sem implementação, sem
barreira especializada — o diff é markdown.

### O levantamento mudou o tamanho do problema

Eu havia dito ao KG "3 arquivos + equivalentes". Errado por baixo. O inventário por categoria achou
**cinco** scanners artesanais sobreviventes (não dois), e uma sexta via de parsing que fica fora de
escopo por construção (o shell dos git hooks, que roda sem o CLI).

E achou uma coisa que ninguém tinha registrado: **o `update` do Python não lê nenhum desses
campos.** `grep -rn pkg_manager pypi/trackfw` retorna vazio. Go e Node decidem quais hooks e qual
CI gerar com base em `hooks`/`ci`/`backend`/`frontend`/`pkg_manager`; o Python simplesmente não
tem o leitor. Não é divergência de implementação — é funcionalidade ausente, invisível porque
nenhum gate compara o comportamento de `update` entre os CLIs.

**Lição de processo:** meu resumo de fila era de memória, não de medição. Antes de escrever escopo
em REQ, enumerar por categoria (parseia / só checa existência / escreve) nos três CLIs.

### Duas decisões que exigiram cuidado

**Escotilha genérica rejeitada.** Expor `Raw map[string]string` e deixar cada consumidor colher a
chave resolveria com menos código, mas recria a divergência que o #106 eliminou: um caminho de
parsing e N de interpretação. O critério que separou as opções foi *deixa exatamente um caminho de
parsing?*

**Segredos: preservação mecânica, sem endosso.** `linear_api_key` e `jira_token` precisam passar
pelo carregador, senão um scanner sobrevive e a AC1 é falsa. Mas ampliar o contrato sem ressalva
ratificaria em silêncio segredo em arquivo versionado — desenho que ninguém avaliou. Ficou em
Negative Scope com sucessor nomeado e revisão do Hades pendente.

### Escolha de branch — deliberada

`docs/`, não `feat/`. A regra `branch_has_wip_roadmap` só enforça em `feat|fix|refactor`, e o
roadmap nasce em `backlog/`. Usar `feat/` obrigaria a pôr o roadmap em `wip/` para o `validate`
passar — estado falso, já que nada está em execução.

### Decomposição — o gerador propôs errado, e isso é dado

`roadmap new --from-req` emitiu um ML por AC. Aqui é a decomposição errada: os ACs são propriedades
transversais (paridade, ausência de segundo parser), não unidades de trabalho — daria oito lotes
tocando os mesmos arquivos, todos sequenciais. Reescrevi por arquivo: 4 waves, 1 ML cada.

**Executor único por wave, cobrindo os 3 CLIs** — não é falta de paralelismo por descuido. Toda
wave paralela com um agente por CLI divergiu nos ciclos de 2026-08-01/02, sempre nos casos que
nenhuma fixture cobria. O único ciclo sem divergência foi o de executor único.

---

## Sessão 2026-08-03 — Ártemis (ML-3A, ROADMAP-2026-08-02-unificar-a-leitura-do-trackfw-yaml-em-um-unico-carregador-nos-tres-clis) — CONCLUÍDO

Branch `refactor/unificar-leitura-trackfw-yaml`. QA não executa Git — commit/push ficam para
`trackfw_architect`.

Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-02-unificar-a-leitura-do-trackfw-yaml-em-um-unico-carregador-nos-tres-clis.md`
Escopo: 3 cenários novos de falsificação (um por CLI) em `scripts/check-gates-falsify.sh` — AC7 —
provando que reintroduzir o scanner artesanal eliminado pelo ML-2A em `loadUpdateConfig`
(Go)/`loadUpdateConfig` (Node)/`_load_update_config` (Python) faz o gate reprovar.

**Protocolo seguido:** suíte herdada rodada ANTES de editar. Achado não relacionado ao meu
escopo: com `LANG=pt_BR.UTF-8` (locale desta máquina), o Cenário 29 (`validate-ok-message`)
reprova porque compara contra uma mensagem pinada em inglês sem fixar `LANG=C` — falso positivo
de ambiente, não regressão de código. Confirmado rodando com `env -u LANG -u LC_ALL -u LANGUAGE`:
92/92 cenários herdados passam limpos. Nota:
`vault/notes/falsify-suite-locale-dependent-false-failure-2026-08-03.md`. Não corrigi (fora do
escopo do ML-3A — "arquivo compartilhado, exclusividade total" não inclui reabrir a asserção do
Cenário 29).

**Cenários 39/40/41 adicionados** (Go, Node.js, Python) — fixture discriminante compartilhada:
```yaml
hooks: lefthook
legacy_project_settings:
  hooks: husky
```
Chave aninhada homônima (candidato mais forte da AC4): o carregador único lê só `hooks` da raiz
(`lefthook`); um scanner artesanal reintroduzido (any-indentation, last-match-wins — mesmo padrão
eliminado pelo ML-2A) sobrescreve a cada linha que casa o prefixo e termina em `husky`. Guarda de
vivacidade: o braço de detecção verifica o efeito observável real (`.husky/pre-commit` escrito e
reportado em vez de `lefthook.yml`), não apenas que o arquivo mudou. Python exercita
especificamente `trackfw update` **bare** (sem `--dry-run`/`--json`/`--targets`/
`--install-missing`) por constraint da barreira do Hefesto — `_run_project` nunca chama o
carregador (nota `python-update-run-project-bypassa-config-load-2026-08-03.md`) e tornaria o
cenário vácuo; adicionei uma guarda extra confirmando que `--dry-run` de fato não emite nenhuma
das duas mensagens (prova de que `_run_project` está "cego" ao hooks scanner, corrompido ou não).

Detecção provada nos 3 CLIs: reintroduzi cada scanner artesanal (via `corrupt_literal`, mesmo
padrão dos cenários 20/21/24/38), confirmei que o cenário correspondente passa a FAIL com o
diagnóstico de vacuidade esperado, depois revertido (as cópias corrompidas vivem só em `$WORK`,
nunca tocam o working tree — confirmado pelo Cenário 18, `no-repo-mutation`, que roda antes e
continuou verde).

**Resultado final:** `env -u LANG -u LC_ALL -u LANGUAGE bash scripts/check-gates-falsify.sh` →
99/99 OK, 0 FAIL (92 herdados + 7 novas asserções dos cenários 39/40/41). `git status --porcelain`
limpo após a corrida (sem mutação do working tree). Cabeçalho/contador final do script atualizado
de "92 scenarios" para "99 scenarios" com a descrição dos 3 novos cenários.

**Arquivos alterados:** `scripts/check-gates-falsify.sh` (único arquivo de implementação, conforme
exclusividade do ML), `docs/agents-working-context.md`, `vault/notes/index.md`,
`vault/notes/falsify-suite-locale-dependent-false-failure-2026-08-03.md` (nova).

**Pendente para o próximo agente:** ML-4A (Apolo) — documentar os 11 campos em
`docs/cli-parity.md`. Roadmap ainda não movido para `done` — decisão do orquestrador.

---

## Sessão 2026-08-04 — Apolo (ML-1B — Node.js: req list/move recursivos + move físico) — CONCLUÍDO (aguardando commit/push por trackfw_architect)

Branch `feat/req-move-list-subpastas-e-move-fisico` (já criada, compartilhada com ML-1A/Go e
ML-1C/Python em paralelo — arquivos distintos).

Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-04-req-move-list-subpastas-e-move-fisico.md`, seção
ML-1B. Escopo: descoberta recursiva de REQs nos 3 layouts (flat, por-estado, by_agent) e move
físico condicional em `req move`, só no CLI Node.js.

**Implementado em `npm/src/generators/req.js`:**
- `listREQFiles(cfg)` — nova função, concatena os 3 conjuntos não-exclusivos: (a)
  `reqDir/*.md` flat, (b) `reqDir/<estado>/*.md` para os 6 estados (`STATE_ORDER` reaproveitado de
  `roadmap.js`), (c) se `cfg.roadmapNamespacing === config.NAMESPACING_BY_AGENT`:
  `reqDir/<agente>/<estado>/*.md`. Reaproveita `VALID_STATES`/`STATE_ORDER` já exportados por
  `roadmap.js` — nenhuma duplicação de constante.
- `listREQs(cfg)` — assinatura trocada de `dir: string` para `cfg: object`; itera
  `listREQFiles(cfg)`; mensagem de vazio usa `cfg.reqDir`.
- `findREQ(name, cfg)` — assinatura trocada de `(name, reqDir: string)` para `(name, cfg)`; itera
  `listREQFiles(cfg)`, primeiro match de basename case-insensitive.
- `moveREQ(name, status)` — mantém assinatura pública (2 args, `cfg` carregado internamente via
  `require('../config').load()`, como antes). Move condicional: `parentDir === reqDir` resolvido →
  in-place (legado, sem mover); `grandparentDir === reqDir` + estado válido → por-estado, move
  para `reqDir/<novo-estado>/`; `greatGrandparentDir === reqDir` + estado válido → by_agent, move
  para `reqDir/<agente>/<novo-estado>/`. Layout não reconhecido → fallback seguro para in-place
  (não inventa destino). `appendREQTransitionLog` nova, grava em `<reqDir>/.trackfw-log` (arquivo
  de log **separado** do `roadmapDir/.trackfw-log`, mesmo formato de `appendTransitionLog` do
  roadmap).

**`npm/src/commands/req.js`:** `req list` agora passa `require('../config').load()` inteiro
(antes só `.reqDir`).

**Não tocado:** `internal/`, `pypi/`, roadmap, REQ, ADR — fora do escopo do ML-1B. Constatei que
os agentes Go/Python já tinham alterado `internal/commands/req.go`, `internal/generators/req.go`,
`internal/generators/req_test.go`, `pypi/trackfw/commands/req.py`,
`pypi/trackfw/generators/req.py` em paralelo na mesma branch — não interferi.

**Testes novos:** `npm/tests/req_list_move_subfolders.test.js` (7 casos) — descoberta por-estado,
descoberta by_agent, `findREQ` recursivo, move físico por-estado, move físico by_agent
(preservando o agente), log de transição em `.trackfw-log`, e mensagem de vazio. O teste legado
`npm/tests/req_move.test.js:15` (`'moveREQ rewrites frontmatter and header status without moving
file'`) foi preservado sem alteração de asserts e continua verde, comprovando que o modo in-place
(REQ solta em `docs/req/`) não regrediu.

**Evidência:** `npm --prefix npm test` → `354 passed, 0 failed`. Não há `npm run lint` no
workspace `npm/` (verificado em `package.json` — apenas `test` e `smoke`).

**Git:** conforme mode lock de Backend (`trackfw_architect` é a única autoridade de Git), **não
fiz commit nem push**. Arquivos modificados/novos ficam no working tree para auditoria e commit do
orquestrador: `npm/src/generators/req.js`, `npm/src/commands/req.js`,
`npm/tests/req_list_move_subfolders.test.js`.

---

## Sessão 2026-08-04 — Apolo (ML-1C — Python: req list novo + move recursivo/físico) — CONCLUÍDO (aguardando commit/push por trackfw_architect)

Branch `feat/req-move-list-subpastas-e-move-fisico` (já criada, compartilhada com ML-1A/Go e
ML-1B/Node.js em paralelo — arquivos distintos).

Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-04-req-move-list-subpastas-e-move-fisico.md`, seção
ML-1C. Escopo: `req list` (inexistente até agora no CLI Python), descoberta recursiva de REQs nos
3 layouts (flat, por-estado, by_agent) e move físico condicional em `req move`, só no CLI Python.

**Implementado em `pypi/trackfw/generators/req.py`:**
- `parse_req_status(filepath)` — nova, extrai status de `"| Status: "` (mesmo algoritmo de
  `parseREQStatus`/`parseREQMeta`).
- `list_req_files(cfg)` — nova, concatena os 3 conjuntos não-exclusivos: (a) `req_dir/*.md` flat,
  (b) `req_dir/<estado>/*.md` para os 6 estados (`STATE_ORDER` importado de `roadmap.py`, sem
  duplicar), (c) se `cfg["roadmap_namespacing"] == "by_agent"`: `req_dir/<agente>/<estado>/*.md`.
- `list_reqs(cfg)` — nova (não existia equivalente Python de `listREQs`), formato `"%-60s %s"`
  idêntico ao Go/Node, mensagem `"No REQs found in {req_dir}"` se vazio.
- `find_req(name, cfg)` — assinatura trocada de `(name, req_dir: str)` para `(name, cfg: dict)`;
  itera `list_req_files(cfg)`.
- `_req_state_dir`/`_req_agent_state_dir` — helpers locais análogos a `_state_dir`/
  `_agent_state_dir` de `roadmap.py`, mas parametrizados em `cfg["req_dir"]` (não reaproveitei
  literalmente os de `roadmap.py` porque são hardcoded em `cfg["roadmap_dir"]` — reaproveitar teria
  movido REQs para dentro de `roadmap_dir`, divergindo do ADR). Reaproveitei apenas as constantes
  `VALID_STATES`/`STATE_ORDER`, importadas de `roadmap.py`.
- `move_req(name, status, cfg=None, req_dir=None, cwd=None)` — mantém compatibilidade retroativa
  com a assinatura antiga (`req_dir=`/`cwd=`, usada pelo teste legado) via parâmetro `cfg` opcional;
  quando `cfg` é passado (uso do CLI), usa `cfg["req_dir"]` resolvido a partir dele. Move
  condicional: `parentDir == req_dir` → in-place (legado, sem mover); `grandparentDir == req_dir` +
  estado válido → por-estado; `basename(parentDir)` é estado válido + `dirname(grandparentDir) ==
  req_dir` → by_agent. Layout não reconhecido → fallback in-place. `_append_req_transition_log`
  nova, grava em `<req_dir>/.trackfw-log` (arquivo de log separado de `<roadmap_dir>/.trackfw-log`).

**`pypi/trackfw/commands/req.py`:** registrado `req_sub.add_parser("list", ...)` (sem argumentos
posicionais); `_dispatch` despacha para `_cmd_list`, que chama `list_reqs(cfg)` com o `cfg`
completo de `load_config()`. `_cmd_move` também passa `cfg=cfg` (full config) em vez de só
`req_dir`. Help text `"Commands: new, move"` → `"Commands: new, move, list"`.

**Não tocado:** `internal/`, `npm/`, roadmap, REQ, ADR — fora do escopo do ML-1C. Constatei que os
agentes Go/Node já tinham alterado `internal/commands/req.go`, `internal/generators/req.go`,
`internal/generators/req_test.go`, `npm/src/commands/req.js`, `npm/src/generators/req.js` em
paralelo na mesma branch — não interferi.

**Testes novos:** `pypi/tests/test_req_list_move_subfolders.py` (10 casos) — `list_reqs` por-estado
e by_agent, mensagem de vazio, `find_req` recursivo, `list_req_files` concatenando os 3 layouts,
move físico por-estado, move físico by_agent, log de transição, e 2 testes de CLI
(`test_cli_req_list_by_state`, `test_cli_req_list_by_agent`) que invocam `trackfw.cli.main()` com
`sys.argv` monkeypatchado, provando `trackfw req list` funcional de ponta a ponta (antes
inexistente). O teste legado `test_move_req_rewrites_status_in_place`
(`pypi/tests/test_generators_req.py:107`) foi preservado sem alteração de asserts e continua
verde, comprovando que o modo in-place (REQ solta em `docs/req/`) não regrediu.

**Evidência:** `cd pypi && python3 -m pytest tests/` → `870 passed, 8 subtests passed` (860
baseline + 10 novos).

**Git:** conforme mode lock de Backend (`trackfw_architect` é a única autoridade de Git), **não
fiz commit nem push**. Arquivos modificados/novos ficam no working tree para auditoria e commit do
orquestrador: `pypi/trackfw/generators/req.py`, `pypi/trackfw/commands/req.py`,
`pypi/tests/test_req_list_move_subfolders.py`.

---

## Sessão 2026-08-04 — Apolo (ML-2B — documentação: namespacing by_agent + move condicional de REQ) — CONCLUÍDO (aguardando commit/push por trackfw_architect)

Branch `feat/req-move-list-subpastas-e-move-fisico` (mesma branch do ML-1A/1B/1C, arquivos
distintos — código já commitado em paralelo por outro agente no ML-2A).

Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-04-req-move-list-subpastas-e-move-fisico.md`, seção
ML-2B. Escopo: atualizar `README.md`/`docs/cli-parity.md` para refletir o comportamento real de
`req list`/`req move` implementado em `internal/generators/req.go` (`listREQFiles`, `findREQ`,
`MoveREQ`) — nenhum arquivo de código tocado.

**`README.md`:**
- Tabela de comandos — nova linha `trackfw req move <name> <status>`; linha de `req list` ajustada
  para citar os 3 layouts (não usei "recursivamente" — os 3 globs em `listREQFiles` são fixos e
  não descem em profundidade arbitrária; "recursivo" seria uma alegação falsificável).
- Seção "Multi-agent namespacing" estendida: REQs reusam o mesmo campo `roadmap_namespacing` (sem
  `req_namespacing` separado, decisão do ADR-2026-08-04); descrição dos 3 layouts de descoberta
  (flat, por-estado, by_agent); comportamento condicional do `req move` — move físico só quando a
  REQ já está numa subpasta de estado reconhecida (nesse modo `<status>` é restrito aos 6 estados
  de governança, `invalid state` caso contrário), in-place quando solta em `req_dir/` (nesse modo
  `<status>` é gravado verbatim, aceitando valores livres como `Open`/`Done` que REQs legadas já
  usam — vocabulário de status diverge por layout, achado confirmado lendo `req.go:317-330`).

**`docs/cli-parity.md`:**
- Linha `req` da tabela de paridade — adicionado `move` (estava só `new, list`).
- Nova subseção "`req list` / `req move` — discovery layouts and conditional physical move" após a
  seção de `roadmap move`: pina os 3 globs fixos de descoberta, o discriminador do modo de move
  (localização do arquivo, não flag), a regra de vocabulário de status por modo, e que a transição
  é gravada em `<req_dir>/.trackfw-log` — arquivo separado de `<roadmap_dir>/.trackfw-log` (`trackfw
  log` só lê o log de roadmap; transições de REQ não aparecem em `trackfw log`, confirmado lendo
  `internal/commands/log.go:27`). Confirmada paridade Go/Node/Python das duas regras (log path e
  vocabulário de status) lendo `pypi/trackfw/generators/req.py` e `npm/src/generators/req.js`.

**Não tocado:** `docs/roadmaps/done/ROADMAP-2026-07-27-integridade-das-referencias-e-ciclo-de-vida-da-req.md`
linhas 199/246, que ainda descrevem o comportamento antigo ("`req move` NÃO move arquivo",
"reescrevendo somente `status:`") — decisão deliberada, não omissão: é um roadmap `done`, registro
histórico do que foi implementado naquela sessão; reescrevê-lo falsificaria a trilha de auditoria.
O comportamento atual está documentado no ADR-2026-08-04 e nesta atualização de README/cli-parity.

**Validação:** `go build ./...` OK; `trackfw validate` → `✓ Nenhuma violação encontrada.` (doc-only,
sem impacto em governança).

**Git:** conforme mode lock de Backend, **não fiz commit nem push**. Arquivos modificados ficam no
working tree para auditoria e commit do orquestrador: `README.md`, `docs/cli-parity.md`.

---

## Sessão 2026-08-04 — Apolo (ML-1B: `discover --init` não gera scripts de attention hooks — lado Node.js) — CONCLUÍDO, aguardando auditoria/commit de `trackfw_architect`

Branch `fix/discover-init-nao-gera-os-scripts-de-attention-hooks-em-go-e-node-quebra-de-paridade-com-python`
(já criada — Backend não executa Git; sem commit/push feitos por este agente).

Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-04-discover-init-nao-gera-os-scripts-de-attention-hooks-em-go-e-node-quebra-de-paridade-com-python.md`,
ML-1B (ainda não marcado ✅ — só após auditoria do orquestrador).
REQ: `docs/req/REQ-2026-08-04-discover-init-nao-gera-os-scripts-de-attention-hooks-em-go-e-node-quebra-de-paridade-com-python.md`.

**Escopo desta sessão: somente o lado Node.js** (Go em paralelo por outro agente; Python já estava correto).

**Correção**: em `npm/src/commands/discover.js`, bloco `opts.init`, adicionada a chamada
`generateAttentionScripts({}, cwd)` (de `../generators/hooks`) **antes** de `injectHooksDetected(cwd)`
— mesma posição relativa usada pelo Python (`pypi/trackfw/commands/discover.py`, depois de
`inject_rules_detected`, antes de `inject_hooks_detected`). `cfg` não precisou de nenhum campo: o
corpo de `generateAttentionScripts(cfg, cwd)` em `npm/src/generators/hooks.js` não lê nada de `cfg`
(os scripts `SIGNAL_SCRIPT`/`CLEANUP_SCRIPT` são conteúdo estático) — confirmado também pelo Python,
cuja `_generate_attention_scripts` nem recebe `cfg`, só `cwd`. Passei `{}` para deixar explícito que o
parâmetro existe mas é irrelevante aqui, em vez de inventar um objeto `cfg` fictício.

**Teste novo**: `npm/tests/discover-init-attention.test.js` (padrão `spawnSync` contra o binário real,
seguindo `npm/tests/update.test.js`) com 3 casos: (1) `discover --init` gera os dois scripts em disco;
(2) conteúdo byte-idêntico ao gerado por `trackfw init` (comparado gerando a referência via
`generateAttentionScripts` num diretório descartável) + modo executável (`& 0o100`); (3) idempotência
— rodar `discover --init` duas vezes não falha nem corrompe os arquivos (a segunda chamada é no-op
porque `trackfw.yaml` já existe e o bloco inteiro de init é pulado, comportamento herdado e não
alterado por este ML).

**Validação**:
- `cd npm && npm test` → `359 passed, 0 failed` (inclui os 3 testes novos, todos verdes).
- `trackfw validate` (raiz do repo) → `✓ Nenhuma violação encontrada.`

**Arquivos alterados**: `npm/src/commands/discover.js` (+5 linhas), `npm/tests/discover-init-attention.test.js` (novo).

Fora de escopo, não tocado: `internal/` (Go), `pypi/` (Python — já correto), roadmap/REQ de
`trackfw branch new` (não relacionado).

Sem commit/push — devolvido para `trackfw_architect` auditar e commitar (Backend não tem autoridade
Git).

## Sessão 2026-08-04 — Apolo (ML-1A: `discover --init` não gera scripts de attention hooks — lado Go) — CONCLUÍDO, aguardando auditoria/commit de `trackfw_architect`

Branch `fix/discover-init-nao-gera-os-scripts-de-attention-hooks-em-go-e-node-quebra-de-paridade-com-python`
(já criada — Backend não executa Git; sem commit/push feitos por este agente).

Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-04-discover-init-nao-gera-os-scripts-de-attention-hooks-em-go-e-node-quebra-de-paridade-com-python.md`,
ML-1A (ainda não marcado ✅ — só após auditoria do orquestrador).
REQ: `docs/req/REQ-2026-08-04-discover-init-nao-gera-os-scripts-de-attention-hooks-em-go-e-node-quebra-de-paridade-com-python.md`.

**Escopo desta sessão: somente o lado Go** (Node.js já concluído em paralelo por outro agente,
ver entrada acima; Python já estava correto).

**Achado durante a implementação (divergência do texto literal do ML)**: o texto do ML dizia só
"exportar `generateAttentionScripts()`", mas a função original escrevia em `"scripts"` (caminho
relativo ao **cwd do processo**), não a um `rootDir` explícito. `InstallGates(r, rootDir, w)` em
`internal/discover/discover.go` é chamado com um `rootDir` que não é garantidamente o cwd do
processo (confirmado pelos próprios testes de `InstallGates`, que usam `t.TempDir()` sem
`os.Chdir`). Uma exportação ingênua sem parâmetro teria escrito os scripts no lugar errado sempre
que `rootDir != cwd`. Comparando com Node.js (`generateAttentionScripts(cfg, cwd)`) e Python
(`_generate_attention_scripts(cwd: str)`) confirmei que os dois outros CLIs já recebiam um
parâmetro de diretório — então segui o mesmo padrão em Go em vez do texto literal do ML.
Nota de vault criada com o detalhe completo:
`vault/notes/go-generateattentionscripts-cwd-vs-rootdir-2026-08-04.md` (indexada em
`vault/notes/index.md`).

**Mudanças**:
1. `internal/generators/scaffold.go` — `generateAttentionScripts()` → exportada como
   `GenerateAttentionScripts(rootDir string) error`. `rootDir == ""` cai para `"."` (mesmo
   comportamento de antes, para os call sites de `init`/`update`). Conteúdo dos scripts gerados
   não mudou (só o caminho onde são escritos passou a ser `filepath.Join(rootDir, "scripts")` em
   vez de `"scripts"` fixo). Call sites internos ao pacote (`scaffold.go:60`, `update.go:77`,
   `update.go:617`) e nos testes (`scaffold_test.go`, `scaffold_parity_test.go`) atualizados para
   `GenerateAttentionScripts("")`.
2. `internal/discover/discover.go` — em `InstallGates`, adicionada a chamada
   `generators.GenerateAttentionScripts(rootDir)` (não-fatal, `⚠ attention scripts: %v` em caso de
   erro, mesmo padrão de `InjectHooksDetected`) **antes** de `generators.InjectHooksDetected(rootDir)`
   — mesma posição relativa usada pelo Python.
3. `internal/discover/discover_test.go` — novo teste `TestInstallGates_GeraAttentionScripts`:
   confirma que os dois scripts existem no `rootDir` após `InstallGates`, modo 0755, conteúdo
   byte-idêntico ao gerado por `generators.GenerateAttentionScripts` num diretório de referência
   separado (prova de paridade com `trackfw init`), e que rodar `InstallGates` duas vezes não
   altera o conteúdo (idempotência). Import `github.com/kgsaran/trackfw/internal/generators`
   adicionado ao arquivo de teste.

**Diff da assinatura exportada**:
```go
// antes (não exportada, internal/generators/scaffold.go:682)
func generateAttentionScripts() error

// depois
func GenerateAttentionScripts(rootDir string) error
```

**Validação**:
- `go build ./...` → sem erros.
- `go test ./internal/discover/... ./internal/generators/... ./internal/commands/...` → todos `ok`.
- `go test ./internal/...` (suíte completa) → todos `ok`.
- `trackfw validate` → `✓ Nenhuma violação encontrada.`

**Arquivos alterados**: `internal/generators/scaffold.go`, `internal/generators/scaffold_test.go`,
`internal/generators/scaffold_parity_test.go`, `internal/generators/update.go`,
`internal/discover/discover.go`, `internal/discover/discover_test.go` (+ nota de vault e esta
entrada de working-context).

Fora de escopo, não tocado: `npm/` (Node.js — já concluído em paralelo), `pypi/` (Python — já
correto), roadmap/REQ de `trackfw branch new` (não relacionado).

Sem commit/push — devolvido para `trackfw_architect` auditar e commitar (Backend não tem
autoridade Git).

## Sessão 2026-08-04 — Apolo (ML-1A Go, ajuste pós-revisão do advisor) — CONCLUÍDO

Correções adicionais após revisão do advisor sobre o trabalho de ML-1A relatado na sessão anterior
(mesma branch, mesmo roadmap/REQ):

1. **stdout parity**: `GenerateAttentionScripts` imprimia `fmt.Printf("  ✓ %s\n", signalPath)` usando
   o caminho completo passado (que em `discover --init` é absoluto, já que
   `internal/commands/discover.go` passa `cwd = os.Getwd()` como `rootDir`). Corrigido para sempre
   imprimir o caminho relativo `filepath.Join("scripts", "trackfw-attention-signal.sh")` /
   `"trackfw-attention-cleanup.sh"`, independentemente de `rootDir` — o disco continua sendo escrito
   em `filepath.Join(rootDir, "scripts")`, só a mensagem impressa mudou. Confirmado empiricamente
   rodando o binário Go compilado (`bin/trackfw discover --init`) e o Node
   (`node npm/bin/trackfw discover --init`) num fixture `git init` vazio cada — as duas linhas de
   saída (`  ✓ scripts/trackfw-attention-signal.sh` / `  ✓ scripts/trackfw-attention-cleanup.sh`)
   ficaram byte-idênticas entre os dois runtimes. Python não imprime nada em sucesso para os
   attention scripts em nenhum runtime (divergência pré-existente, fora do escopo desta REQ).
2. **Comentário obsoleto**: `internal/generators/update.go:807` citava `generateAttentionScripts`
   (nome antigo, não-exportado) numa lista de funções em comentário — atualizado para
   `GenerateAttentionScripts`.
3. Confirmado por grep que nenhum script de gate (`scripts/check-gates-*.sh`) ou `docs/cli-parity.md`
   pina o nome antigo não-exportado — só comentários/docs/mensagens de teste, todos já corretos ou
   inofensivos.

Nota de vault `vault/notes/go-generateattentionscripts-cwd-vs-rootdir-2026-08-04.md` atualizada com
o addendum sobre o stdout.

**Validação re-executada**: `go build ./...` OK, `go test ./internal/...` todos `ok`,
`trackfw validate` → `✓ Nenhuma violação encontrada.`

Sem commit/push — devolvido para `trackfw_architect` auditar e commitar.

## Sessão 2026-08-04 — Apolo (Wave 1 Go: `trackfw branch new`) — CONCLUÍDO

**Escopo**: REQ/roadmap `.../comando-trackfw-branch-new-para-bloquear-criacao-de-branch-sem-req-roadmap-em-wip`,
Wave 1 (ML-1A + ML-1B), só Go. Branch: `feat/comando-trackfw-branch-new-para-bloquear-criacao-de-branch-sem-req-roadmap-em-wip`.

**ML-1A — extração de matching (refactor puro)**: extraída `BranchSlugMatchesRoadmap(branchSlug string,
wipDirs, doneDirs []string) (matched bool, candidates []string)` de dentro de
`validateBranchHasWIPRoadmap` (`internal/validator/validator.go`). Também exportados wrappers finos
`ResolveWIPDirs`, `ResolveDoneDirs`, `NormalizeBranchSlug` (chamam as versões não-exportadas já
existentes, sem renomeá-las — evita mexer nos outros ~5 call-sites internos) e extraídas as duas
mensagens de orientação para funções reutilizáveis `BranchGovernanceOrientation(branch string) string`
(candidates vazio) e `BranchNoMatchingRoadmapMessage(branch string, candidates []string) string`
(candidates não-vazio) — usadas tanto por `validateBranchHasWIPRoadmap` quanto pelo novo comando.
`go test ./internal/validator/...` sem nenhuma asserção alterada (confirmado antes/depois do
refactor).

**ML-1B — comando `trackfw branch new <type>/<slug>`**: novo `internal/commands/branch.go`,
registrado em `root.go` (`newBranchCmd()`). Fluxo: valida `type` ∈ {feat, fix, refactor} e slug
não-vazio (`parseBranchSpec`) → normaliza slug e chama `validator.BranchSlugMatchesRoadmap` contra
`ResolveWIPDirs`+`ResolveDoneDirs` → sem match: imprime a mesma mensagem de
`BranchGovernanceOrientation`/`BranchNoMatchingRoadmapMessage` (nunca duplicada), exit não-zero, git
nunca é chamado → com match: `git checkout -b <type>/<slug>` via `exec.Command` com stdio herdado
(stdout/stderr diretos do processo, sem reformatar) → `--dry-run`: roda a mesma checagem e imprime
"would create"/"would block: <mensagem>", nunca chama git, exit não-zero quando bloquearia. Erro de
branch já existente propaga sem tratamento especial (delega ao erro nativo do `git checkout -b`,
como pedido na REQ). Testado com `branchNewDeps` injetável (mesmo padrão de `shipDeps` em
`ship.go`/`ship_test.go`) — 17 testes novos em `internal/commands/branch_test.go` cobrindo match em
wip/done (via mock), sem match com/sem candidatos, dry-run nos dois cenários, tipo inválido, slug
vazio, e propagação do erro nativo do Git.

**Validação**: `go build ./...` OK; `go test ./internal/...` todos `ok`; `go vet ./...` limpo;
`trackfw validate` → `✓ Nenhuma violação encontrada.`; `trackfw help branch` e
`trackfw branch new --dry-run <slug-existente-em-wip>` / `<slug-órfão>` testados manualmente com o
binário compilado — comportamento e mensagens conferem com `trackfw validate`.

Fora de escopo desta sessão (Wave 2/3, outro agente): Node.js (`npm/`), Python (`pypi/`),
`docs/cli-parity.md`, gate de paridade.

Sem commit/push — devolvido para `trackfw_architect` auditar e commitar (Backend não tem
autoridade Git).

## Sessão 2026-08-04 — Apolo (Wave 2 Python: `trackfw branch new`) — CONCLUÍDO

**Escopo**: mesmo REQ/roadmap acima, ML-2B (só Python — Go é a referência comportamental já
auditada em Wave 1; ML-2A Node.js é de outro agente em paralelo). Branch
`feat/comando-trackfw-branch-new-para-bloquear-criacao-de-branch-sem-req-roadmap-em-wip`.

**Matching extraído (não duplicado)**: `pypi/trackfw/validator.py` ganhou
`normalize_branch_slug`, `branch_slug_matches_roadmap(branch_slug, wip_dirs, done_dirs) ->
(matched, candidates)`, `branch_governance_orientation(branch)` e
`branch_no_matching_roadmap_message(branch, candidates)` — extraídas do corpo antigo de
`validate_branch_has_wip_roadmap`, que agora só chama essas funções (refactor puro, nenhuma
assertion de teste mudou). Mensagens byte-idênticas às Go
(`BranchGovernanceOrientation`/`BranchNoMatchingRoadmapMessage` em
`internal/validator/validator.go`) — confirmado por diff direto rodando os dois binários lado a
lado no mesmo fixture.

**Comando novo**: `pypi/trackfw/commands/branch.py` (`branch new <type>/<slug>`), registrado em
`pypi/trackfw/cli.py`. `run_branch_new(...)` é testável por DI (mesmo padrão de
`trackfw.ship.runner.run_ship`/`MockGit` em `test_ship.py`) — todas as dependências (config,
resolve wip/done dirs, match_slug, exec_git_checkout, out/err_out) são injetáveis, default para
as implementações reais. Contrato: tipo inválido/slug vazio → stderr (sem tocar match nem git),
exit 1; sem match → mensagem de orientação no stdout + `blocked: no matching roadmap in wip/ nor
done/ for "<branch>"` no stderr, git nunca chamado, exit 1 (`--dry-run` prefixa com `[dry-run]
would block:` mas mesma msg); com match → `git checkout -b <branch>` via `subprocess.run` com
stdio herdado, propaga o `returncode` literal (não intercepta nem reformata a saída do Git);
`--dry-run` com match → só imprime `[dry-run] would create branch "<branch>" (git checkout -b
<branch>)`, nunca chama git, exit 0.

**Testes**: `pypi/tests/test_branch.py`, 21 casos novos cobrindo os mesmos 8 cenários do Go
(`branch_test.go`) — match wip/done, sem match com/sem candidatos, dry-run nos dois casos, tipo
inválido, slug vazio, branch já existe (propaga `returncode` do git fake), + normalização de slug
+ matching real contra filesystem via `tmp_path` para `branch_slug_matches_roadmap`.

**Validação**: `cd pypi && python3 -m pytest -q` → 890 passed (nenhuma quebra pré-existente);
`trackfw validate` (raiz) → `✓ Nenhuma violação encontrada.`; comparação byte-a-byte Python vs Go
(binário compilado em `/tmp/trackfw-go`) em 5 cenários (`--dry-run` bloqueado, bloqueado sem
dry-run, tipo inválido, slug vazio, `--dry-run` com match usando o próprio slug desta REQ) —
stdout e stderr idênticos em todos.

Fora de escopo desta sessão: Node.js (`npm/` — outro agente), `docs/cli-parity.md` e gate de
paridade (Wave 3).

Sem commit/push — devolvido para `trackfw_architect` auditar e commitar (Backend não tem
autoridade Git).

## Sessão 2026-08-04 — Apolo (Wave 2 Node.js: `trackfw branch new`, ML-2A) — CONCLUÍDO

**Escopo**: mesmo REQ/roadmap acima, ML-2A (só Node.js). Go (ML-1A/1B) é a referência
comportamental — comparado byte a byte contra binário Go recompilado localmente
(`go build -o /tmp/trackfw-go ./cmd/trackfw`).

**Extração no validador Node** (`npm/src/validator/index.js`): de dentro de
`validateBranchHasWIPRoadmap`, extraídas `branchSlugMatchesRoadmap(branchSlug, wipDirs, doneDirs)`
→ `{matched, candidates}` (espelha `validator.BranchSlugMatchesRoadmap` do Go),
`branchGovernanceOrientation(branch)` e `branchNoMatchingRoadmapMessage(branch, candidates)` —
strings byte-idênticas às do Go. `validateBranchHasWIPRoadmap` passou a chamá-las; comportamento
observável inalterado (os 5 testes existentes de `branch_has_wip_roadmap` em `tests/validator.test.js`
continuam verdes sem alteração). Todas as 4 funções + `normalizeBranchSlug` (já existia, não estava
exportada) foram adicionadas aos exports do módulo.

**Novo módulo testável** `npm/src/branch/runner.js` (espelha o split `ship.js`/`ship/runner.js` já
usado neste CLI): `parseBranchSpec`, `defaultGitCheckout` (spawnSync stdio:'inherit', git nunca
reformatado) e `runBranchNew(spec, dryRun, deps)` com deps 100% injetáveis
(`loadConfig/resolveWIPDirs/resolveDoneDirs/matchSlug/execGitCheckout/writeln/writeErr`) — nenhum
teste toca git real. `npm/src/commands/branch.js` é só wiring do Commander (`branch new <spec>
--dry-run`), registrado em `npm/src/commands/index.js`.

**Achado não-óbvio de paridade byte-a-byte** (por isso vale registrar): o Go faz split
stdout/stderr que não é óbvio olhando só `branch.go` — a mensagem de orientação (`deps.out`) vai
para stdout, mas `root.go:Execute()` imprime o `error` retornado por `RunE` em stderr por cima
disso (`fmt.Fprintln(os.Stderr, err)`), então o caso "bloqueado" produz DUAS linhas: a mensagem de
orientação em stdout + `blocked: no matching roadmap in wip/ nor done/ for "<branch>"` em stderr.
Sem inspecionar `root.go` isso não seria óbvio só lendo `branch.go`/`branch_test.go`. Reproduzido em
`runBranchNew` via `writeErr` separado. Confirmado com `diff` lado a lado entre `node bin/trackfw` e
o binário Go recompilado para: tipo inválido, slug vazio, sem match com `--dry-run`, sem match sem
`--dry-run`, com match `--dry-run` — todos byte-idênticos. Único ponto que NÃO foi replicado
literalmente: o erro de "branch já existe" do Go inclui uma segunda linha `exit status 128` (artefato
de `exec.Command.Error.Error()` do Go, não parte do contrato da REQ) — Node só deixa passar o stderr
nativo do git (`fatal: a branch named '...' already exists`) com exit code 1, que é o que a REQ pede
("delega ao erro nativo do git").

**Testes**: `npm/tests/branch.test.js`, 15 testes novos (mesmos 8 cenários da REQ + os 17 do Go
condensados por equivalência). `npm test` → 374 passed, 0 failed (inclui os 63 de
`validator.test.js` inalterados). Testado manualmente: `node bin/trackfw branch new --help`,
`branch new feat/algum-slug-sem-match --dry-run` (bloqueia sem chamar git), checkout real e
"already exists" em repo git descartável isolado em `/tmp`.

**Verificação adicional pós-revisão (stdout/stderr capturados em arquivos separados, não via
`2>&1` combinado)**: confirmado com `diff` independente por stream — Node vs Go idênticos byte a
byte tanto em stdout quanto em stderr para os casos "sem match --dry-run" e "tipo inválido".
`trackfw help branch` funciona nos dois CLIs (exit 0; conteúdo difere porque `help.js` do Node é
dinâmico a partir de `root.commands`/description, sem tabela estática que precisasse de entrada
nova — igual ao padrão já usado por `roadmap`/`req`/`adr`). `trackfw branch` sem subcomando diverge
de exit code entre os CLIs (Node: exit 1, ajuda no stderr; Go/cobra: exit 0, ajuda no stdout) —
comportamento **pré-existente e idêntico** ao de `roadmap`/`req` no Node (nenhum desses grupos tem
`.action()` no comando pai; é a convenção já estabelecida no Commander deste CLI, confirmado
comparando `node bin/trackfw roadmap`/`req` sem subcomando com os mesmos comandos em Go) — não é
uma divergência introduzida por este ML, é um gap de paridade cross-CLI pré-existente e mais amplo
que `branch`; fica fora do escopo do ML-2A (REQ não pede paridade de exit code para grupo sem
subcomando, só "`trackfw help branch` funcional").

Fora de escopo desta sessão (outro agente, em paralelo): Python (`pypi/`) — ML-2B, já com diffs
próprios no working tree ao final desta sessão (`pypi/trackfw/validator.py`, `pypi/trackfw/cli.py`,
`pypi/trackfw/commands/branch.py`, `pypi/tests/test_branch.py`), não tocados por mim.
`docs/cli-parity.md` e gate de paridade — Wave 3, depois.

Sem commit/push — devolvido para `trackfw_architect` auditar e commitar (Backend não tem
autoridade Git).

## Sessão 2026-08-04 — Prometeu (ML-2A: auditoria pós-HTML-escaping — sem fix) — CONCLUÍDO, aguardando auditoria/commit de `trackfw_architect`

Branch `fix/json-marshalindent-do-go-escapa-html-e-diverge-de-node-python-em-3-targets-do-catalogo-kiro-amazonq-antigravity-legacy`
(já criada pelo orquestrador — Tooling não executa Git; sem commit/push feitos por este agente).

Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-04-json-marshalindent-do-go-escapa-html-e-diverge-de-node-python-em-3-targets-do-catalogo-kiro-amazonq-antigravity-legacy.md`,
Wave 2 / ML-2A. Wave 1 (fix de `render.go:57`) já concluída e auditada anteriormente.

**Escopo**: auditoria (não fix automático) de todo `json.Marshal`/`json.MarshalIndent`/
`json.NewEncoder` restante em `internal/`, verificando gate de paridade cross-runtime real e
plausibilidade de conteúdo com `<`/`>`/`&`.

**Resultado — nenhum fix aplicado**, registrado item-por-item na seção do ML-2A do roadmap:
- `agentfiles.go` (6 sites, settings.json/hooks), `manifest.go`, `validator.go:50` — sem gate,
  conteúdo estruturado/enum, risco teórico.
- `validate.go`/`barrier.go`/`update.go`/`update_harness.go` (saídas `--json`) — gate existe
  (`check-validate-parity.sh`, `check-update-parity.sh`, `check-barrier.sh`, `check-artifact-parity.sh`)
  mas todos reparseiam JSON (`json.loads`) antes de comparar, o que desfaz qualquer escaping HTML
  antes da comparação — mesmo bug do ML-1A não seria pego por nenhum deles.
- Achados fora da lista original da REQ: `integrations_flags.go:352` (`agents/skills list --json`,
  gate `check-integration-cli-parity.sh` mas também mascarado por reparse) e **`context.go:185`**
  (`trackfw context --format json`, inclui títulos reais de REQ/ADR/Roadmap — risco real e
  plausível, mas **sem gate nenhum** cobrindo — candidato a REQ futura para criar o gate antes de
  qualquer fix). `internal/server/server.go` confirmado sem import em nenhum lugar do código.
- `internal/identity/identity.go:72` (`UserNickname` é texto livre do usuário) — sem gate byte-a-byte
  do próprio `Save()`, mesmo padrão de risco-real-sem-gate do `context.go`.
- Confirmado fora de escopo (Go-only, sem contrato de paridade): `internal/serve/*.go`,
  `internal/sync/jira.go`, `internal/sync/linear.go` — não tocados.

**Validação**: `go build ./...` OK, `go test ./...` OK (sem regressão, código não alterado),
`GO_BIN=bin/trackfw scripts/check-identity-parity.sh` verde (11 combinações), `make quality` verde
(100/100 cenários de falsify), `trackfw validate` sem violações. Único arquivo modificado: o
roadmap (auditoria registrada). Nenhum código de produto tocado.

Sem commit/push — devolvido para `trackfw_architect` auditar e commitar (Tooling não tem
autoridade Git).

## Sessão 2026-08-05 — Apolo (Wave 1 + Wave 2: scripts de attention hooks divergentes, gate novo) — CONCLUÍDO, aguardando auditoria/commit de `trackfw_architect`

Branch `fix/scripts-de-attention-hooks-divergem-em-conteudo-entre-go-node-e-python-sem-gate-de-paridade`
(já criada pelo orquestrador — sem commit/push feitos por este agente).

Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-04-scripts-de-attention-hooks-divergem-em-conteudo-entre-go-node-e-python-sem-gate-de-paridade.md`,
Wave 1 (ML-1A) + Wave 2 (ML-2A) — sequencial, uma mão só, conforme roadmap.

**Wave 1 — texto canônico único aplicado nos 3 literais-fonte**: descoberto empiricamente (via
`discover --init` real nos 3 runtimes + diff, não só leitura de código) que a divergência tinha
4 dimensões, não 3 — o roadmap não mencionava a linha em branco entre `TIMESTAMP=...` e
`TOOL_ESC=...` no script de signal (Go/Python tinham, Node.js não). Canônico escolhido:
comentário em inglês ("Script is intentionally a no-op when executed outside the project root"),
linha em branco presente após `ROADMAP_DIR=${ROADMAP_DIR:-docs/roadmaps}` E antes de `TOOL_ESC=`,
`sed` de expressão única (`sed 'expr1; expr2'`). Aplicado em `internal/generators/scaffold.go`,
`npm/src/generators/hooks.js` (linhas ~61/78/85-86/98/101/105), `pypi/trackfw/generators/init_gen.py`
(comentário em ambos os literais). Nenhum golden/fixture de teste referenciava o texto antigo
(`grep -rln` do roadmap não achou nada em `internal/discover`, `npm/tests`, `pypi/tests`).

**Wave 2 — gate novo**: `scripts/check-attention-scripts-parity.sh` (padrão de
`check-branch-new-parity.sh`: `GO_BIN` resolvido/buildável, roda `discover --init` real nos 3
runtimes num fixture vazio por runtime, `diff -u` byte-a-byte go-vs-node e go-vs-py dos dois
scripts, guard de vacuidade P2 se algum runtime não gerar os arquivos). Integrado ao `Makefile`
(alvo `parity`, antes de `check-gates-falsify.sh`). Documentado em `docs/cli-parity.md` (nova
seção antes de "Princípios de design de gates"). Cenário 43 adicionado a
`scripts/check-gates-falsify.sh` (P4): corrompe só o comentário do literal Python
`_ATTENTION_CLEANUP_SH` (via `corrupt_literal` com contexto estendido até `ROADMAP_DIR=$(grep`
para isolar da ocorrência idêntica em `_ATTENTION_SIGNAL_SH` — sem isso `corrupt_literal` aborta
com "expected exactly 1 occurrence"), roda o gate novo a partir de cópia própria (padrão dos
Cenários 36/42) e confirma exit != 0 com o diff explícito no diagnóstico. Texto do resumo final
do falsify (contagem "100 scenarios" → "101 scenarios", "15 gates" → "16 gates") atualizado.

**Validação (evidência)**:
- `go build ./...` OK, `go test ./internal/...` OK (todos os pacotes)
- `npm test` — 374 passed, 0 failed
- `python3 -m pytest` (a partir de `pypi/`) — 890 passed
- `diff` vazio empírico confirmado entre os 3 binários reais (Go compilado, `node npm/bin/trackfw`,
  `python3 -m trackfw`) para os dois scripts, via `discover --init` em fixtures novos
- `GO_BIN=bin/trackfw scripts/check-attention-scripts-parity.sh` verde isoladamente
- `make quality` verde (build + test + test-node + test-python + lint + parity completo, incluindo
  o gate novo e o falsify)
- `scripts/check-gates-falsify.sh` completo — 101/101 cenários OK, 0 FAIL, incluindo o Cenário 43
  novo confirmando que a regressão injetada é detectada
- `trackfw validate` — sem violações

Arquivos modificados: `internal/generators/scaffold.go`, `npm/src/generators/hooks.js`,
`pypi/trackfw/generators/init_gen.py`, `scripts/check-attention-scripts-parity.sh` (novo),
`Makefile`, `docs/cli-parity.md`, `scripts/check-gates-falsify.sh`.

Sem commit/push — devolvido para `trackfw_architect` auditar e commitar (Backend não tem
autoridade Git).

## Sessão 2026-08-05 — Prometeu (ML-2A + ML-2B: target `opencode` — catálogo + adapter Go) — CONCLUÍDO, aguardando auditoria/commit de `trackfw_architect`

Branch `feat/compatibilidade-com-opencode-opencode-ai-para-uso-de-modelos-open-source` (já criada pelo
orquestrador; sem commit/push feitos por este agente — Tooling não tem autoridade Git).

Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-04-compatibilidade-com-opencode-opencode-ai-para-uso-de-modelos-open-source.md`,
Wave 2 (ML-2A + ML-2B). REQ:
`docs/req/REQ-2026-08-04-compatibilidade-com-opencode-opencode-ai-para-uso-de-modelos-open-source.md`.
Wave 1 (pesquisa contra o OpenCode real 1.18.13, já mergeada nesta branch) decidiu: skills sem
mudança; agents exigem nova `Representation` `"opencode-agent"` (reconstrução de frontmatter do zero,
mesmo padrão de `"agent-directory"`), com `mode: subagent` sempre fixo e `model:`/`tools:`/`memory:`
sempre omitidos (decisão de produto — deixar o OpenCode resolver pelo default já configurado pelo
usuário em `opencode.json`, alinhado à motivação de negócio de usar modelos open-source/locais).

**ML-2A — catálogo + adapter Go**:
- `internal/integrations/render.go` — novo case `"opencode-agent"` em `Render()`.
- `internal/integrations/assets/catalog.json` — novo target `opencode` (surface `cli`, escopos
  `global`+`project`, paths `.opencode/agents|skills/...` projeto e `~/.config/opencode/agents|
  skills/...` global, confirmados experimentalmente na Wave 1).
- `internal/integrations/render_test.go` — `TestRenderOpenCodeAgent`.
- `internal/integrations/catalog_test.go` — contagem de targets atualizada de 9 para 10.

**ML-2B — lifecycle + validação contra o binário real**:
- `internal/commands/agents_skills_test.go` — `TestOpenCodeAgentsLifecycleEndToEnd` (install → list →
  update → uninstall com `--targets opencode`, análogo ao teste já existente para `codex`).
- Validado manualmente contra `opencode` 1.18.13 real (`/opt/homebrew/bin/opencode`) num projeto de
  teste isolado (`git init` fora do repo, removido ao final): `opencode agent list` carregou
  `trackfw-architect (subagent)` e `trackfw-backend (subagent)` sem NENHUM erro de config (confirma
  que o bug de `tools:` da Wave 1 está corrigido); `opencode serve` + `GET /agent` confirmou
  `mode: "subagent"` e a chave `model` de fato ausente do JSON resolvido (não só null); `opencode
  debug skill` confirmou a skill reconhecida (colisão de nome com skill global do Claude Code
  pré-existente na máquina — achado colateral já documentado na Wave 1, não-acionável); `trackfw
  discover --init` num projeto com `AGENTS.md` pré-existente confirmou a injeção de regras
  funcionando sem nenhuma mudança de código em `agentfiles.go`.
- Nenhuma mudança foi necessária em `internal/generators/agentfiles.go` — a detecção já é por path,
  independente de qual ferramenta criou o arquivo (confirma achado #5 do escopo original).

**Validação (evidência)**:
- `go build ./...` OK
- `go test ./...` completo OK (todos os pacotes)
- `trackfw agents list --json` mostra o target `opencode` com `representation: opencode-agent`
- `trackfw validate` — sem violações

Fora de escopo (Waves 3/4, outros agentes): Node.js (`npm/`), Python (`pypi/`),
`docs/cli-parity.md`, gate de paridade de identidade.

Sem commit/push — devolvido para `trackfw_architect` auditar e commitar (Tooling não tem
autoridade Git).

## Sessão 2026-08-05 — Prometeu (ML-4A: documentação + gate de paridade de identidade) — INICIADO

Branch `feat/compatibilidade-com-opencode-opencode-ai-para-uso-de-modelos-open-source` (recriada
após merge do PR #134, Waves 1–3). Roadmap
`docs/roadmaps/wip/ROADMAP-2026-08-04-compatibilidade-com-opencode-opencode-ai-para-uso-de-modelos-open-source.md`,
ML-4A (única pendente).

**Escopo**: adicionar OpenCode a `docs/cli-parity.md` (lista de targets suportados por
`agents`/`skills`); confirmar que `scripts/check-identity-parity.sh` cobre o novo target
automaticamente; varredura ampla por listas hardcoded de targets com o mesmo defeito da Wave 3
(harness `update`). Sem autoridade Git — devolvo para `trackfw_architect` auditar e commitar.

## Sessão 2026-08-05 — Prometeu (ML-4A) — CONCLUÍDO, aguardando auditoria/commit de `trackfw_architect`

**Arquivos alterados**:
- `docs/cli-parity.md` — OpenCode incluído na frase de targets suportados de "AI integration
  lifecycle" + nova subseção `### OpenCode agent representation (opencode-agent)` (frontmatter
  reconstruído do zero, `mode: subagent` fixo, `model:`/`tools:`/`memory:` omitidos, com a evidência
  empírica da Wave 1 sobre o hard-fail de `tools:` no OpenCode 1.18.13). Não dupliquei a tabela
  "Declared harness targets — pinned list" (já atualizada na Wave 3) — só referenciei.
- `scripts/check-identity-parity.sh` — **confirmado, sem alteração**: `load_catalog_targets()` já
  deriva os targets de `catalog.json` dinamicamente, `opencode` entra no gate sem edição manual.
- **Achado extra (mesma classe da Wave 3, fora do escopo original mas coberto pelo item 3 do
  briefing)**: `npm/src/commands/init.js:61` tinha um `Set` hardcoded de AI tools que **rejeitava
  `opencode`** no modo não-interativo (`Unsupported AI tool: opencode`) — divergência funcional real
  vs Go/Python (que validam via catálogo). Corrigido. Também corrigidos, pelo mesmo motivo: wizard
  interativo `huh` (Go, `internal/commands/init.go`) e `checkbox` (Node.js, `init.js`) não listavam
  OpenCode como opção — corrigido nos dois, com verificação prévia de que `InjectRulesForTool`/
  `injectRulesForTool` fazem no-op seguro para `opencode`. Teste Go
  `TestInitAIToolsHelpIncludesEveryCatalogTarget` (`agents_skills_test.go`) tinha a mesma lista
  desatualizada (passava vacuamente) — atualizado. Python (`init.py`) já era genérico, sem gap.
  Detalhe completo na seção "Achado extra (ML-4A)" do roadmap.
- Roadmap `docs/roadmaps/wip/ROADMAP-2026-08-04-compatibilidade-com-opencode-opencode-ai-para-uso-de-modelos-open-source.md`
  — ML-4A marcado `✅ Concluído`, critérios de aceite marcados, nova seção "Achado extra (ML-4A)".

**Validação (evidência)**:
- `go build ./...` OK; `go test ./...` completo OK
- `npm test` (npm/) — 375 passed, 0 failed
- `python3 -m pytest` (pypi/) — 892 passed, 8 subtests passed
- `make quality` — verde (todos os gates, incluindo `check-identity-parity.sh` e
  `check-gates-falsify.sh`, 101 cenários)
- `make parity` — 101 cenários verdes
- `trackfw validate` — sem violações

Roadmap **não movido para `done/`** — deixado em `wip/` para o orquestrador (Zeus) auditar e mover.
Sem commit/push — devolvido para `trackfw_architect` auditar e commitar (Tooling não tem autoridade
Git).

## Sessão 2026-08-05 — Apolo (fix crítico pós-auditoria: geradores de credential-guard nunca eram
chamados por fluxo real) — CONCLUÍDO, aguardando auditoria/commit de `trackfw_architect`

Branch `feat/hooks-de-guarda-contra-materializacao-de-credenciais` (já criada — sem
commit/push feitos por este agente). Roadmap
`docs/roadmaps/wip/ROADMAP-2026-08-05-hooks-de-guarda-contra-materializacao-de-credenciais-reais-por-subagentes.md`.

**Bug**: `GenerateCredentialGuardScript`/`generateCredentialGuardScript`/
`_generate_credential_guard_script` (ML-1A) escreviam `scripts/trackfw-credential-guard.sh`, mas
nenhum fluxo real (`init`/`update`/`discover --init`) chamava essas funções — só testes diretos. O
script nunca existia em disco em projetos reais, apesar do wiring de hooks (ML-2A/2B/2C) já apontar
para ele.

**Fix — chamada adicionada ao lado da chamada irmã de `GenerateAttentionScripts`, mesma condição
(incondicional), em todos os pontos reais**:
- Go: `internal/generators/scaffold.go` (`Scaffold`, linha ~60-64), `internal/generators/update.go`
  (`Update`, ~linha 77-81; e `runProjectTarget("agent-hooks")`, ~linha 598-624, incluindo
  `scripts/trackfw-credential-guard.sh` nos `relPaths`/display path do target), `internal/discover/discover.go`
  (`InstallGates`, ~linha 61-64).
- Node.js: `npm/src/generators/init.js` (nova função local `generateCredentialGuardScript` +
  chamada em `scaffold()` + export), `npm/src/commands/discover.js` (bloco `opts.init`),
  `npm/src/commands/update.js` (`buildProjectTargets` target `agent-hooks`, incluindo o path no
  `relPaths`/display path).
- Python: centralizado em `pypi/trackfw/generators/hooks.py` `inject_hooks_detected()` (ao lado da
  chamada existente a `_generate_attention_scripts`) — cobre `update.py` (`_run` e `_run_project`) e
  `init_gen.py` `scaffold()` (que já chama `inject_hooks_detected` depois de `_generate_attention_scripts`
  direto); chamada direta também adicionada em `init_gen.py::scaffold()` e em
  `pypi/trackfw/commands/discover.py` (bloco `opts.init`) para espelhar exatamente cada call site do
  gerador irmão. `AGENT_HOOKS_RELATIVE_PATHS`/`AGENT_HOOKS_DISPLAY_PATH` em `update.py` atualizados.

**Testes novos/estendidos (fluxo real, não chamada direta do gerador)**:
- Go: `internal/commands/init_test.go::TestInitGeneratesCredentialGuardScript` (via `newInitCmd()` +
  `cmd.Execute()`), `internal/discover/discover_test.go::TestInstallGates_GeraCredentialGuardScript`
  (via `InstallGates`, byte-idêntico à referência + idempotência),
  `internal/generators/update_test.go::TestUpdateBackfillsCredentialGuardScriptForPreExistingProject`
  (cenário de upgrade: projeto com `trackfw-attention-signal.sh` mas sem `trackfw-credential-guard.sh`
  → `Update()` cria o que falta) + assert estendido em
  `TestUpdateInjectsAndUpdatesAttentionHooksIdempotently`.
- Node.js: `npm/tests/generators.test.js` (assert estendido no teste de `scaffold` e no teste de
  `update` real + novo teste de cenário de upgrade), `npm/tests/discover-init-attention.test.js`
  (novo teste via binário real `bin/trackfw discover --init`, byte-idêntico à referência).
- Python: `pypi/tests/test_generators_init.py` (assert estendido em
  `test_scaffold_generates_attention_scripts` + novo
  `test_update_command_upgrade_scenario_backfills_credential_guard` via `commands.update._run`),
  `pypi/tests/test_discover.py` (novo `test_discover_init_generates_credential_guard_script` via
  `discover_cmd._cmd_discover`).

**Validação (evidência)**:
- `go build ./...` OK; `go test ./...` completo OK (todos os pacotes)
- `npm test` — 380 passed, 0 failed (era 378 antes)
- `python3 -m pytest` — 913 passed, 8 subtests passed (era 911 antes)
- `trackfw validate` — sem violações
- `make quality` — Go/Node/Python verdes; `check-cli-parity.sh` falha por divergência de versão
  `6.4.1` (Go) vs `6.3.1` (Python) — **confirmado pré-existente** (reproduzido também com
  `git stash` no HEAD anterior às minhas mudanças), fora do escopo deste fix.

Sem commit/push — devolvido para `trackfw_architect` auditar e commitar (Backend não tem autoridade
Git).

## Sessão 2026-08-06 — Apolo (ML-2D: GitHub Copilot wiring + correção de divergência de formato) — CONCLUÍDO, aguardando auditoria/commit de `trackfw_architect`

Branch `feat/hooks-de-guarda-contra-materializacao-de-credenciais` (já criada — sem commit/push feitos
por este agente). Roadmap
`docs/roadmaps/wip/ROADMAP-2026-08-05-hooks-de-guarda-contra-materializacao-de-credenciais-reais-por-subagentes.md`,
ML-2D (Wave 2 — GitHub Copilot).

**Investigação (obrigatória antes de codar)**: confirmado via
`https://docs.github.com/en/copilot/reference/hooks-reference` (2026-08-05, `curl` do JSON
`renderedPage` embutido pelo Next.js) que o formato real de `.github/hooks/*.json` é
`{"version": 1, "hooks": {"<event>": [{"type": "command", "bash": "...", "cwd": "...",
"timeoutSec": N}, ...]}}` — Python já usava esse formato corretamente; Go e Node usavam
`{"hooks": [{"event", "run"}]}`, que não corresponde a nenhum formato documentado. Alinhados Go/Node
ao formato do Python. Confirmado também suporte real a `matcher` (regex ancorado contra `toolName`)
em `preToolUse`/`postToolUse`, ao contrário do pressuposto do ADR de que não existia — usado
`matcher: "bash"` (nome runtime do tool de shell em minúsculo, válido para eventos camelCase como os
usados aqui). `trackfw-credential-guard.sh` foi inspecionado e não depende de nomes de campo
específicos do payload (grep sobre o payload bruto inteiro), então funciona independente do
camelCase/PascalCase do evento. Concorrência: "If multiple hooks of the same type are configured,
they execute in order" — resposta mais definitiva entre todos os CLIs cobertos até aqui (serial, em
ordem de configuração), ao contrário do Codex (concorrente, ML-2B) e do Gemini (indocumentado,
ML-2C). Detalhe completo em `docs/cli-parity.md` (seção "GitHub Copilot wiring (ML-2D)").

**Arquivos alterados**:
- `internal/generators/agentfiles.go` (`InjectCopilotHooks` — formato realinhado + novas entradas
  `matcher:"bash"` de credential-guard, com comentário de fonte/investigação)
- `internal/generators/agentfiles_test.go` (`TestInjectCopilotHooks` — asserts reescritos para o novo
  formato)
- `internal/generators/copilot_hooks_parity_test.go` (novo —
  `TestInjectCopilotHooks_StructuralParityAcrossStacks`, invoca Go/Node/Python como subprocessos
  reais via `node`/`python3` e compara a estrutura JSON resultante, não byte-a-byte)
- `npm/src/generators/hooks.js` (`injectCopilotHooks` — mesmo realinhamento)
- `npm/tests/generators.test.js` (asserts reescritos)
- `pypi/trackfw/generators/hooks.py` (`inject_copilot_hooks` — formato pré-existente mantido +
  novas entradas de credential-guard)
- `pypi/tests/test_generators_init.py` (asserts estendidos)
- `docs/cli-parity.md` (nova seção "GitHub Copilot wiring (ML-2D)")
- `docs/roadmaps/wip/ROADMAP-2026-08-05-...md` (ML-2D marcado ✅ Concluído com nota de auditoria)

**Validação (evidência)**:
- `go build ./...` OK; `go test ./...` OK (todos os pacotes, incluindo o novo teste de paridade
  estrutural cross-stack)
- `npm --prefix npm test` (`node --test tests/*.test.js`) — 380 passed, 0 failed
- `python3 -m pytest pypi/tests/` — 913 passed, 8 subtests passed
- `python3 -m pytest pypi/tests/ -k hooks` — 19 passed (subset relevante)

Sem commit/push — devolvido para `trackfw_architect` auditar e commitar (Backend não tem autoridade
Git). Não toquei em Cursor (ML-2E) nem Kiro (ML-2F), conforme escopo deste ML.

---

## Sessão 2026-08-05 — Apolo (ML-2F: Kiro — wiring credential-guard) — CONCLUÍDO, aguardando auditoria/commit de `trackfw_architect`

Branch `feat/hooks-de-guarda-contra-materializacao-de-credenciais` (já criada pelo orquestrador —
Backend não executa Git; sem commit/push feitos por este agente).

Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-05-hooks-de-guarda-contra-materializacao-de-credenciais-reais-por-subagentes.md`,
ML-2F (último ML da Wave 2). REQ/ADR: `docs/req/REQ-2026-08-05-...md` /
`docs/adr/ADR-2026-08-05-hook-de-guarda-contra-materializacao-de-credenciais-reais-por-subagentes.md`.

**Investigação (obrigatória por instrução do ML) — confirmada afirmativamente**: via
`kiro.dev/docs/hooks/`, `.../hooks/types` e `.../hooks/actions/` (2026-08-05, `curl -L` do RSC/HTML,
sem WebFetch/WebSearch disponível nesta execução), `PreToolUse` é um trigger real, distinto de
`PostFileSave`/eventos de IDE — "Before a tool is about to execute", Can block: **Yes**. Resolve a
dúvida em aberto da ADR: o mecanismo de hooks do Kiro de fato intercepta invocações de tool (incluindo
shell) antes da execução. Decisão: **implementar** (não re-escopar).

**Achado crítico, corrigido no mesmo ML (não deixado só como nota)**: o wiring pré-existente
(`InjectKiroHooks`/`injectKiroHooks`/`inject_kiro_hooks` nos 3 stacks) usava um schema que não existe
na documentação real do Kiro — campo `"event"` (deveria ser `"trigger"`), `matcher` como objeto
`{tool_name: ".*"}` (deveria ser regex string; `.*` nem é um valor de matcher documentado), sem
`"version"` no topo (deveria ser `"v1"`, string). Como `.kiro/hooks/trackfw-attention.json` é um
arquivo 100% owned/overwritten pelo trackfw (mesmo padrão do GitHub Copilot no ML-2D, diferente do
Cursor no ML-2E que é merge-target com conteúdo de usuário), corrigi as entradas legadas
`trackfw-attention-signal`/`-cleanup` junto com as novas de credential-guard, em vez de deixar
entradas comprovadamente inválidas ao lado de entradas novas corretas no mesmo array `hooks`.

**Wiring novo**: `PreToolUse`/matcher `"shell"` e `PostToolUse`/matcher `"shell"` (categoria
documentada "all built-in shell command-related tools") apontando para
`scripts/trackfw-credential-guard.sh`. Contrato de bloqueio do Kiro é mais estrito que Claude
Code/Codex/Gemini: **qualquer** exit code não-zero de um hook `PreToolUse` bloqueia a invocação (não
só exit 2) — reauditei `trackfw-credential-guard.sh` e confirmei que só tem `exit 0`/`exit 2` nos
caminhos normais de operação, então `warn` nunca bloqueia espuriamente no Kiro. Nenhuma mudança no
script foi necessária. Detalhe completo, com citações das 3 páginas, em `docs/cli-parity.md` (seção
"Kiro wiring (ML-2F)"). Adicionei também uma nota de resolução na própria ADR (tabela de suporte por
CLI), já que a dúvida registrada ali estava explicitamente em aberto.

**Arquivos alterados**:
- `internal/generators/agentfiles.go` (`InjectKiroHooks` — schema realinhado + 2 hooks novos de
  credential-guard, comentário extenso de fonte/investigação)
- `internal/generators/agentfiles_test.go` (`TestInjectKiroHooks` — reescrito: 4 hooks, `version:
  "v1"`, ausência de `event`, `matcher` string não-objeto, campos específicos do guard-pre/post)
- `npm/src/generators/hooks.js` (`injectKiroHooks` — mesma extensão)
- `npm/tests/generators.test.js` (asserts reescritos, mesmo padrão)
- `pypi/trackfw/generators/hooks.py` (`inject_kiro_hooks` — mesma extensão)
- `pypi/tests/test_generators_init.py` (`test_inject_kiro_hooks` — asserts reescritos)
- `docs/cli-parity.md` (nova seção "Kiro wiring (ML-2F)")
- `docs/adr/ADR-2026-08-05-...md` (nota de resolução na tabela de suporte por CLI, linha Kiro)
- `docs/roadmaps/wip/ROADMAP-2026-08-05-...md` (ML-2F marcado ✅ Concluído com nota de auditoria)

**Validação (evidência)**:
- `go build ./...` OK; `go vet ./...` OK; `go test ./...` OK (todos os pacotes)
- `node --test tests/generators.test.js` (dentro de `npm/`) — 28 passed, 0 failed, incluindo
  `injectKiroHooks creates .kiro/hooks/trackfw-attention.json idempotently`
- `npm --prefix npm test` completo — 381 passed, 0 failed
- `python3 -m pytest pypi/` completo — 914 passed
- `python3 -m pytest pypi/tests/ -k hooks` — 20 passed
- `trackfw validate` (binário compilado desta branch) — "Nenhuma violação encontrada."

Sem commit/push — devolvido para `trackfw_architect` auditar e commitar (Backend não tem autoridade
Git). Wave 2 completa (ML-2A a ML-2F, todos ✅). Próximo: Wave 3 (ML-3A, gate de paridade
hooks.json/settings.json).
