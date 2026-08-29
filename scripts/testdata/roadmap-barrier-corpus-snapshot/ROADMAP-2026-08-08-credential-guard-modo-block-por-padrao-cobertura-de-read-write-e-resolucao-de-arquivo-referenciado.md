---
status: done
date: 2026-08-08
req: "docs/req/REQ-2026-08-08-credential-guard-modo-block-cobertura-read-write-e-resolucao-de-arquivo-referenciado.md"
squad: ""
---

# Roadmap: credential-guard — modo block por padrão, cobertura de Read/Write e resolução de arquivo referenciado

> Created: 2026-08-08 | Status: done

## Context
REQ: docs/req/REQ-2026-08-08-credential-guard-modo-block-cobertura-read-write-e-resolucao-de-arquivo-referenciado.md
ADR emendada (Wave 0 já concluída pelo orquestrador antes deste roadmap): docs/adr/ADR-2026-08-06-hooks-de-credential-guard-em-escopo-global-via-trackfw-update-harness.md — itens 6, 7, 8.

3 lacunas a fechar, nos 3 CLIs (Go/Node/Python), 6 alvos de CLI cada (Claude/Codex/Gemini/Copilot/Cursor/Kiro):
1. Fallback do modo global de `warn` para `block` (ADR emenda 6).
2. Wiring de matchers Read/Write/Edit por CLI (ADR emenda 7 — tabela de matchers por CLI).
3. Segunda camada de detecção via conteúdo de arquivo referenciado (ADR emenda 8).

## Acceptance Criteria
- [x] Fallback do MODE global (sem `trackfw.yaml`, ou `trackfw.yaml` sem `credential_guard.mode`) passa a `block` nos 3 stacks; `trackfw.yaml` com `credential_guard.mode` explícito continua respeitado (warn ou block), inclusive quando o hook global é disparado a partir do cwd desse projeto.
- [x] Wiring Read/Write/Edit adicionado para Claude, Gemini, Kiro, Copilot, Cursor (matchers da tabela da ADR emenda 7); Codex documentado como limitação (sem matcher de leitura dedicado), com `apply_patch`/`Edit`/`Write` cobrindo escrita.
- [x] Segunda camada de detecção lê conteúdo de arquivo referenciado (via redirect capturado por `REDIRECTS`, e via argumento de arquivo existente quando o comando é `cat`/`head`/`tail`/`jq`/`grep`), com teto de tamanho para não ler arquivos grandes.
- [x] Testes novos nos 3 stacks cobrindo os 3 cenários do REQ.
- [x] `make quality` sem regressão, paridade Go-Node-Python mantida.
- [x] `docs/cli-parity.md` atualizado com a limitação do Codex (item de wiring) e a mudança de default de modo.

## Wave 1 — Script core: default block + segunda camada de detecção (3 MLs em paralelo, 1 por stack)
> Dependências: nenhuma (ADR já emendada). Arquivos independentes entre MLs desta wave — paralelismo real.

### ML-1A — Go: fallback block + detecção por conteúdo de arquivo
**Status:** ✅ Concluído
**Arquivos afetados:** `internal/generators/scaffold.go`
**Ações:**
1. Em `credentialGuardGlobalTail` (linha ~980): substituir `MODE="warn"` fixo por lógica que reusa a
   mesma leitura de `credential_guard.mode` de `trackfw.yaml` que `credentialGuardProjectTail` já faz
   (linha ~931: `grep -A 5 '^credential_guard:' trackfw.yaml | grep 'mode:' | ...`), SEM exigir
   `trackfw.yaml` existir (ao contrário da variante de projeto, que faz `[ -f "trackfw.yaml" ] || exit 0`
   antes — a variante global não tem esse guard e não deve ganhá-lo aqui). Fallback de `case "$MODE" in
   warn|block) ;; *) MODE="..." ;; esac` passa de `"warn"` para `"block"`.
2. Extrair essa lógica de resolução de MODE (grep do trackfw.yaml + case + fallback) para uma nova
   constante Go compartilhada (ex.: `credentialGuardModeResolution`) usada tanto por
   `credentialGuardProjectTail` (fallback `warn` — comportamento inalterado, guard de
   `[ -f trackfw.yaml ]` já filtra o caso "sem projeto") quanto por `credentialGuardGlobalTail`
   (fallback `block`, sem esse guard). Parametrizar o fallback via variável de shell definida antes do
   include (ex.: `DEFAULT_MODE="warn"` / `DEFAULT_MODE="block"` concatenada antes do bloco comum) —
   evita duplicar a linha de `grep` em dois lugares.
3. Em `credentialGuardDetectionCore` (linha ~866): após o bloco de `REDIRECTS`/`is_ephemeral_target`
   existente, adicionar segunda camada: se `MATCH` ainda vazio (não achou no payload cru), (a) para
   cada `target` em `REDIRECTS` que não for exemplo (helper já existe), se o arquivo existir e for
   menor que um teto (ex. `[ "$(wc -c < "$target" 2>/dev/null || echo 0)" -lt 1048576 ]`), escanear seu
   conteúdo com os mesmos `JWT_PATTERN`/`AWS_KEY_PATTERN`; (b) extrair o nome do comando (primeiro
   token não-vazio de `RAW`) e, se for um de `cat|head|tail|jq|grep`, escanear os demais tokens do
   comando: para cada token que seja um caminho de arquivo regular existente (`[ -f "$token" ]`) e
   dentro do mesmo teto de tamanho, escanear seu conteúdo. Se qualquer escaneamento achar padrão,
   setar `MATCH` normalmente (segue o fluxo já existente de warn/block).
4. Atualizar o comentário de `GenerateCredentialGuardScript`/`GenerateGlobalCredentialGuardScript` e o
   comentário de `credentialGuardGlobalTail` (linhas 966-972, que hoje documentam "sempre warn") para
   refletir a nova decisão — apontar para a emenda 6 da ADR-2026-08-06 em vez de descrever `warn` fixo
   como decisão vigente.
**Critérios de aceite:**
- [ ] `go build ./...` sem erros
- [ ] `MODE` global resolve para `block` quando não há `trackfw.yaml` no cwd
- [ ] `MODE` global resolve para o valor explícito de `credential_guard.mode` quando presente (warn ou block)
- [ ] `head -c 50 /tmp/x` com `/tmp/x` contendo um JWT é capturado
- [ ] comportamento de `credentialGuardProjectTail` inalterado para quem já define `mode: warn` explícito

### ML-1B — Node: fallback block + detecção por conteúdo de arquivo
**Status:** ✅ Concluído
**Arquivos afetados:** `npm/src/generators/hooks.js`
**Ações:** Mesma lógica de ML-1A, portada para as constantes `CG_GLOBAL_TAIL` (linha ~265),
`CG_PROJECT_TAIL` (linha ~230) e `CG_DETECTION_CORE` (linha ~167). Atenção ao escaping de `\` em
template literals JS (duplo-escapado em relação à string raw do Go/Python — já documentado como risco
de manutenção na pesquisa desta REQ).
**Critérios de aceite:**
- [ ] equivalentes aos de ML-1A, validados via `npm test` (arquivo `npm/tests/credential_guard.test.js`)
- [ ] conteúdo funcional idêntico ao gerado pelo Go após parse do shell (mesmo teste de paridade de
  string usado hoje, se existir em `credential_guard_dedup_test.go`/equivalente)

### ML-1C — Python: fallback block + detecção por conteúdo de arquivo
**Status:** ✅ Concluído
**Arquivos afetados:** `pypi/trackfw/generators/init_gen.py`
**Ações:** Mesma lógica de ML-1A, portada para `_CG_GLOBAL_TAIL` (linha ~948), `_CG_PROJECT_TAIL`
(linha ~913) e `_CG_DETECTION_CORE` (linha ~850). Usar raw string (`r"""..."""`) como já é o padrão do
arquivo.
**Critérios de aceite:**
- [ ] equivalentes aos de ML-1A, validados via `pytest` (arquivo `pypi/tests/test_credential_guard.py`)

## Wave 2 — Wiring Read/Write/Edit por CLI (2 MLs em paralelo — Node fica fora desta wave, ver nota)
> Dependências: nenhuma (arquivos distintos de Wave 1 para Go/Python). Node faz core+wiring no mesmo
> arquivo (`hooks.js`) — para não ter dois agentes editando o mesmo arquivo em paralelo, o wiring do
> Node é feito como continuação de ML-1B (mesmo agente, mesmo ML, sequencial dentro do próprio ML —
> ver adendo em ML-1B abaixo) em vez de um ML-2B separado.

**Adendo a ML-1B (Node):** depois dos passos de detecção, o mesmo agente adiciona, nas funções
`injectClaudeHooks` (linha ~456), `injectGeminiHooks` (~528), `injectKiroHooks` (~570),
`injectCopilotHooks` (~641), `injectCursorHooks` (~725) de `npm/src/generators/hooks.js`, as entradas
de wiring da tabela abaixo (mesma tabela usada em ML-2A/ML-2C). Codex (`injectCodexHooks`, ~488) só
ganha comentário documentando a limitação — nenhum matcher de leitura novo.

### ML-2A — Go: wiring de matchers Read/Write/Edit
**Status:** ✅ Concluído
**Arquivos afetados:** `internal/generators/agentfiles.go`
**Ações:** Para cada `InjectXHooks`, adicionar entradas de `PreToolUse`/`PostToolUse` (ou evento
equivalente do CLI) apontando para `scripts/trackfw-credential-guard.sh`, usando os matchers:

| CLI | Read | Write/Edit | Onde |
|---|---|---|---|
| `InjectClaudeHooks` | `Read` | `Write\|Edit` | mesmo padrão de `mergeClaudeHookArray` já usado para `"Bash"` (linha ~219) |
| `InjectCodexHooks` | **não adicionar** — comentário documentando limitação (sem tool de leitura interceptável) | `apply_patch` (aliases `Edit`/`Write` já documentados) | função em ~261 |
| `InjectGeminiHooks` | `read_file\|read_many_files` | `write_file\|replace` | `BeforeTool`/`AfterTool`, função em ~357 |
| `InjectKiroHooks` | `read` | `write` | novas entradas `trigger: PreToolUse/PostToolUse`, função em ~470 |
| `InjectCopilotHooks` | `view` | `create\|edit` | novas entradas em `preToolUse`/`postToolUse`, função em ~563 |
| `InjectCursorHooks` | `Read` | `Write` | via eventos genéricos `preToolUse`/`postToolUse` (distintos de `beforeShellExecution`/`afterShellExecution`), função em ~674 |

Aplicar o mesmo padrão de dedup já existente (`globalCredentialGuardInstalledX()`) para cada nova
entrada — não duplicar quando o wiring global já cobre o CLI.
**Critérios de aceite:**
- [ ] `go build ./...` sem erros
- [ ] `go test ./internal/generators/...` verde
- [ ] cada `InjectXHooks` gera as novas entradas de matcher conforme tabela
- [ ] Codex: nenhuma entrada de matcher de leitura é gerada; comentário explica a limitação

### ML-2C — Python: wiring de matchers Read/Write/Edit
**Status:** ✅ Concluído
**Arquivos afetados:** `pypi/trackfw/generators/hooks.py`
**Ações:** Mesma tabela e mesma lógica de ML-2A, portada para `inject_claude_hooks` (~240),
`inject_codex_hooks` (~315, só comentário), `inject_gemini_hooks` (~381), `inject_kiro_hooks` (~430),
`inject_copilot_hooks` (~498), `inject_cursor_hooks` (~607).
**Critérios de aceite:**
- [ ] equivalentes aos de ML-2A, validados via `pytest` (`pypi/tests/test_generators_init.py` +
  cobertura nova em `pypi/tests/test_credential_guard.py`)

## Wave 3 — Testes novos (3 MLs em paralelo, depende de Wave 1 + Wave 2 do mesmo stack)
> Dependências: ML-3A depende de ML-1A+ML-2A; ML-3B depende de ML-1B (que já inclui o wiring Node);
> ML-3C depende de ML-1C+ML-2C.

### ML-3A — Go: testes dos 3 cenários
**Status:** ✅ Concluído
**Arquivos afetados:** `internal/generators/credential_guard_test.go`, `internal/generators/agentfiles_test.go`
**Ações:** Adicionar casos: (a) `trackfw.yaml` ausente/sem `credential_guard.mode` → script global
bloqueia (exit 2) num payload com JWT; (b) fixture de payload `Read`/`Write`/`Edit` com JWT/AWS key no
`tool_input` → `InjectXHooks` (Claude/Gemini/Kiro/Copilot/Cursor) grava a entrada de matcher esperada;
(c) comando `head -c 50 arquivo-com-jwt.txt` (sem o JWT literal no comando) → script captura via a
nova segunda camada de detecção.
**Critérios de aceite:**
- [ ] `go test ./internal/generators/... -run CredentialGuard` verde, cobrindo os 3 cenários

### ML-3B — Node: testes dos 3 cenários
**Status:** ✅ Concluído
**Arquivos afetados:** `npm/tests/credential_guard.test.js`, `npm/tests/generators.test.js`
**Ações:** Mesmos 3 cenários de ML-3A.
**Critérios de aceite:**
- [ ] `npm test` verde, cobrindo os 3 cenários

### ML-3C — Python: testes dos 3 cenários
**Status:** ✅ Concluído
**Arquivos afetados:** `pypi/tests/test_credential_guard.py`, `pypi/tests/test_generators_init.py`
**Ações:** Mesmos 3 cenários de ML-3A.
**Critérios de aceite:**
- [ ] `pytest` verde, cobrindo os 3 cenários

## Wave 4 — Gate de qualidade e paridade (1 ML, sequencial, após Waves 1-3)
> Dependências: Wave 1 + Wave 2 + Wave 3 completas.

### ML-4A — make quality + docs/cli-parity.md
**Status:** ✅ Concluído
**Arquivos afetados:** `docs/cli-parity.md`, `scripts/check-agent-hooks-parity.sh`,
`docs/req/REQ-2026-08-08-credential-guard-modo-block-cobertura-read-write-e-resolucao-de-arquivo-referenciado.md`,
`vault/notes/check-agent-hooks-parity-unisolated-home-false-failure-2026-08-08.md`,
`vault/notes/index.md`
**Ações:**
1. Rodar `make quality` (Go + Node.js + Python + contratos de paridade) na raiz do repo.
2. Atualizar `docs/cli-parity.md` com: (a) mudança de default do modo global (`warn`→`block`); (b)
   cobertura de matchers Read/Write/Edit por CLI, incluindo a limitação documentada do Codex.
3. Corrigir qualquer divergência de paridade apontada pelo gate antes de considerar o roadmap concluído.
**Critérios de aceite:**
- [x] `make quality` sem erros/regressão
- [x] `docs/cli-parity.md` reflete o estado final dos 3 stacks

**Resultado:** `make quality` falhou inicialmente em `scripts/check-agent-hooks-parity.sh` (18 FAIL,
`credential-guard-present`, idênticos nos 3 stacks) — causa raiz ambiental, não regressão de
código: o gate roda `discover --init` sem isolar `$HOME`, e nesta máquina o credential-guard global
já está instalado, então o dedup do ML-3A pula silenciosamente a entrada de projeto (mesma causa
raiz documentada em
`vault/notes/node-global-credential-guard-dedup-breaks-inject-tests-on-real-home-2026-08-08.md` para
um teste Node, agora encontrada num gate shell). Corrigido isolando `HOME` por runtime em
`run_discover_init` (mesmo padrão já usado em `check-update-parity.sh`), sem alterar nenhum
comportamento de produto. Nota nova:
`vault/notes/check-agent-hooks-parity-unisolated-home-false-failure-2026-08-08.md`. Após o fix,
`make quality` → 0 `FAIL`, exit 0 (103/103 cenários de falsificação, todos os gates de paridade
OK). `docs/cli-parity.md`: duas seções novas — "Modo default do credential-guard GLOBAL — `warn` →
`block`" (supersede o achado transversal #1 desatualizado da seção "Suporte por CLI — visão
consolidada, escopo GLOBAL") e "Cobertura de matchers Read/Write/Edit do credential-guard por CLI"
(tabela dos 6 CLIs, cada matcher confirmado lendo `internal/generators/agentfiles.go`). Gap do
ML-3B (`roadmap:` vazio no frontmatter da REQ e sem linha `Roadmap:` no corpo) corrigido nos dois
lugares — `trackfw validate` confirmado limpo (`✓ Nenhuma violação encontrada.`, exit 0) antes de
concluir este ML.
