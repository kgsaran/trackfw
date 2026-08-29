---
status: done
date: 2026-08-14
req: "docs/req/REQ-2026-08-14-roteamento-de-model-tier-no-render-de-agentes-para-codex-toml-e-cursor-frontmatter.md"
squad: ""
---

# Roadmap: roteamento de model tier no render de agentes para codex (toml) e cursor (frontmatter)

> Created: 2026-08-14 | Status: done

## Context
<!-- Derived from REQ: REQ-2026-08-14-roteamento-de-model-tier-no-render-de-agentes-para-codex-toml-e-cursor-frontmatter.md -->
REQ: docs/req/REQ-2026-08-14-roteamento-de-model-tier-no-render-de-agentes-para-codex-toml-e-cursor-frontmatter.md
ADR: docs/adr/ADR-2026-08-14-roteamento-de-model-tier-por-alvo-no-render-de-agentes-para-codex-e-cursor.md

O catálogo canônico já declara `model: opus` (architect) / `model: sonnet` (demais 9
agentes) no frontmatter de `internal/integrations/assets/agents/*.md`. Esse tier só é
efetivo hoje para Claude Code (passthrough nativo) e Antigravity (`mapModel()`,
já implementado). Codex nunca emite `model` no TOML gerado; Cursor emite `model: opus`/
`model: sonnet` verbatim, valores não documentados como aceitos pela Cursor. Este
roadmap fecha os dois gaps, mantendo `gemini`/`kiro` (mesma representação
`agent-markdown` do Cursor) bit-a-bit inalterados por não terem sintaxe de modelo
confirmada.

**Investigação de código já feita nesta sessão (não repetir):**
- Go: `Render()` em `internal/integrations/render.go`, chamado uma única vez em
  `internal/integrations/plan.go:55` — `target.ID` já está disponível nesse call site,
  só não é passado adiante.
- Node.js: `render()` em `npm/src/integrations/render.js:238`, chamado em
  `npm/src/integrations/index.js:58` — o call site **já passa** `target: targetEntry.id`
  no objeto; a função só não o desestrutura nem usa.
- Python: `render()` em `pypi/trackfw/integrations/renderers.py:225` **já recebe**
  `target: str` como parâmetro posicional (chamado em
  `pypi/trackfw/integrations/catalog.py:108`) — nenhuma mudança de assinatura
  necessária aqui, só usar o parâmetro que já existe.

## Acceptance Criteria
- [x] `target.ID`/`target` chega ao render em Go, Node.js e Python (Go via novo
      parâmetro; Node.js consumindo o campo já enviado; Python via parâmetro já
      existente).
- [x] Branch `custom-agent-toml` (Codex) emite `model = "..."` mapeado a partir do
      tier canônico, nos 3 CLIs.
- [x] Branch `agent-markdown` (Cursor) emite `model: ...` mapeado a partir do tier
      canônico **apenas quando o alvo é `cursor`**, nos 3 CLIs.
- [x] `gemini` e `kiro` continuam bit-a-bit idênticos ao comportamento atual —
      coberto por teste de regressão explícito.
- [x] `make quality` (Go + Node.js + Python + contratos de paridade) verde.

## Wave 1 — Threading do target ID até o Render (Go + Node.js; Python sem mudança de assinatura)
> Dependências: nenhuma. As 3 MLs tocam arquivos/linguagens distintas — executar em paralelo.

### ML-1A — Go: `Render()` recebe `targetID string`
**Status:** ✅ Concluído
**Arquivos afetados:**
- `internal/integrations/render.go`
- `internal/integrations/plan.go`
- `internal/integrations/render_test.go`
**Ações:**
1. Em `render.go`, mudar a assinatura de
   `func Render(item Item, kind ItemKind, capability Capability, source []byte, cfg identity.Config) ([]byte, error)`
   para
   `func Render(item Item, kind ItemKind, capability Capability, source []byte, cfg identity.Config, targetID string) ([]byte, error)`
   (novo parâmetro ao final, para minimizar diff nos call sites existentes).
2. Atualizar o comentário-doc da função para mencionar o novo parâmetro e seu uso
   (diferenciar `cursor` de `gemini`/`kiro` dentro de `agent-markdown` nas Waves
   seguintes — não implementar a diferenciação ainda nesta ML).
3. Em `plan.go:55`, mudar a chamada para
   `Render(item, request.Kind, capability, source, request.Identity, target.ID)`.
4. Em `render_test.go`, atualizar todas as chamadas diretas a `Render(...)` para passar
   o `targetID` correspondente ao cenário testado (usar o `target.ID` real do
   catálogo em cada subteste, nunca uma string arbitrária).
**Critérios de aceite:**
- [ ] `go build ./...` sem erros
- [ ] `go test ./internal/integrations/...` verde
- [ ] Nenhum output de render muda nesta ML (mudança é só de assinatura/plumbing)
**Comandos de validação:** `go build ./... && go test ./internal/integrations/...`

### ML-1B — Node.js: `render()` consome o `target` já enviado pelo call site
**Status:** ✅ Concluído
**Arquivos afetados:**
- `npm/src/integrations/render.js`
**Ações:**
1. Na linha 238, mudar a desestruturação de
   `function render({ kind, content, capability, item, identity: cfg })`
   para
   `function render({ kind, content, capability, item, identity: cfg, target })`.
2. Não alterar `npm/src/integrations/index.js` — o call site (linha 58) já envia
   `target: targetEntry.id`, nenhuma mudança necessária ali.
3. Se algum teste chamar `render({...})` sem o campo `target` para os branches que
   ainda não o consultam (Wave 1 não introduz uso de `target`), nenhum ajuste é
   necessário — `target` fica `undefined` sem quebrar nada até a Wave 2/3.
**Critérios de aceite:**
- [ ] `npm test` (workspace raiz ou `--workspace` do pacote de integrações) verde
- [ ] Nenhum output de render muda nesta ML
**Comandos de validação:** `npm test --workspace=trackfw` (ajustar nome do workspace
conforme `npm/package.json`)

### ML-1C — Python: confirmar que `target` já chega ao render (sem mudança de assinatura)
**Status:** ✅ Concluído
**Arquivos afetados:**
- `pypi/tests/test_integrations_identity.py` (novo teste, sem alterar `renderers.py`
  nem `catalog.py` nesta ML)
**Ações:**
1. Adicionar um teste que chama
   `render("agents", "cursor", "ide", item, source, capability, identity_cfg)` e
   `render("agents", "gemini", "cli", item, source, capability, identity_cfg)`
   confirmando que a função aceita o parâmetro `target` posicional hoje (já aceita —
   este teste é uma prova negativa: documenta o estado atual antes das Waves 2/3
   mudarem o comportamento, para servir de baseline de regressão).
**Critérios de aceite:**
- [ ] `pytest pypi/tests/test_integrations_identity.py` verde
- [ ] Nenhuma mudança em `renderers.py` ou `catalog.py` nesta ML
**Comandos de validação:** `cd pypi && python -m pytest tests/test_integrations_identity.py`

## Wave 2 — Codex: emitir `model` no TOML do custom agent
> Dependências: Wave 1 completa (Go e Node.js precisam do `targetID`/`target`
> chegando; Python já o tinha, mas mantido nesta wave por coerência de revisão).
> As 3 MLs tocam arquivos/linguagens distintas — executar em paralelo.

**Mapeamento de modelo (mesmo nos 3 CLIs, valores citados no ADR vinculado, fonte:
documentação Codex CLI pesquisada em 2026-08-14 — `majesticlabs.dev/blog/202607/codex-cli-configuration-guide`):**
- `opus` → `gpt-5.4`
- `sonnet` → `gpt-5.4-mini`
- qualquer outro valor (ou ausente) → omitir `model` (mesmo padrão de fallback de
  `mapModel()`/`_map_model` para Antigravity)

> ⚠️ IDs de modelo Codex são versionados e mudam com o ciclo de release da OpenAI —
> confirmar contra `codex --version` / documentação vigente do usuário antes de
> fechar esta wave como Done (ver ADR, seção Consequences).

### ML-2A — Go: `mapModelCodex()` + branch `custom-agent-toml`
**Status:** ✅ Concluído
**Arquivos afetados:**
- `internal/integrations/render.go`
- `internal/integrations/render_test.go`
**Ações:**
1. Adicionar função `mapModelCodex(model string) (string, bool)` logo após
   `mapModel()` (linha ~364), com o mesmo formato de retorno
   (`(valor mapeado, true)` ou `("", false)` para omitir), usando os valores do
   mapeamento acima.
2. No `case "custom-agent-toml":` (linha ~53), adicionar `model` ao TOML gerado
   quando `mapModelCodex(model)` retornar `ok == true`:
   ```go
   lines := []string{
       fmt.Sprintf("name = %s", strconv.Quote(strings.ReplaceAll(name, "-", "_"))),
       fmt.Sprintf("description = %s", strconv.Quote(description)),
   }
   if mapped, ok := mapModelCodex(model); ok {
       lines = append(lines, fmt.Sprintf("model = %s", strconv.Quote(mapped)))
   }
   lines = append(lines, fmt.Sprintf("developer_instructions = %s", strconv.Quote(body)))
   return []byte(strings.Join(lines, "\n") + "\n"), nil
   ```
   (adaptar formatação ao estilo já usado na função; o essencial é `model` aparecer
   entre `description` e `developer_instructions`, condicionado ao mapeamento).
3. Testes: adicionar caso em `render_test.go` cobrindo `targetID: "codex"` para um
   agente `architect` (`model: opus` no asset) esperando `model = "gpt-5.4"` no TOML,
   e para `backend` (`model: sonnet`) esperando `model = "gpt-5.4-mini"`.
**Critérios de aceite:**
- [ ] `go build ./...` sem erros
- [ ] `go test ./internal/integrations/...` verde, incluindo os novos casos
- [ ] `.codex/agents/trackfw-architect.toml` gerado por `trackfw init --ai-tools codex`
      contém `model = "gpt-5.4"`; `trackfw-backend.toml` contém `model = "gpt-5.4-mini"`
**Comandos de validação:** `go build ./... && go test ./internal/integrations/...`

### ML-2B — Node.js: `_mapModelCodex()` + branch `custom-agent-toml`
**Status:** ✅ Concluído
**Arquivos afetados:**
- `npm/src/integrations/render.js`
**Ações:**
1. Espelhar `mapModelCodex()` do Go como função `mapModelCodex(model)` (ou nome
   consistente com o estilo já usado no arquivo para `mapModel`, se existir
   equivalente Node — verificar se o Antigravity mapping já tem um nome de função
   em `render.js` e seguir a mesma convenção de nomenclatura).
2. Aplicar o mesmo mapeamento de valores (`opus→gpt-5.4`, `sonnet→gpt-5.4-mini`) no
   branch `custom-agent-toml`, na mesma posição relativa (entre `description` e
   `developer_instructions`).
3. Adicionar/atualizar teste equivalente ao ML-2A em
   `npm/tests/render_opencode.test.js` (ou novo arquivo `npm/tests/render_codex.test.js`
   se o padrão do projeto for um arquivo por representação — seguir convenção
   existente).
**Critérios de aceite:**
- [ ] `npm test` verde, incluindo os novos casos
- [ ] Output byte-a-byte idêntico ao gerado pelo Go para o mesmo input (contrato de
      paridade)
**Comandos de validação:** `npm test --workspace=trackfw`

### ML-2C — Python: `_map_model_codex()` + branch `custom-agent-toml`
**Status:** ✅ Concluído
**Arquivos afetados:**
- `pypi/trackfw/integrations/renderers.py`
**Ações:**
1. Adicionar função `_map_model_codex(model: str) -> str | None` logo após
   `_map_model()` (linha ~47), mesmo formato de retorno (`None` = omitir).
2. No bloco `if representation == "custom-agent-toml":` (linha ~254), inserir a
   linha `model = "..."` entre `description` e `developer_instructions` quando
   `_map_model_codex(metadata.get("model", ""))` não for `None`.
3. Adicionar/atualizar teste equivalente em `pypi/tests/` (arquivo de teste de render
   já existente para `custom-agent-toml`, ou novo se não houver).
**Critérios de aceite:**
- [ ] `pytest` do pacote `pypi` verde, incluindo os novos casos
- [ ] Output byte-a-byte idêntico ao gerado pelo Go para o mesmo input
**Comandos de validação:** `cd pypi && python -m pytest`

## Wave 3 — Cursor: emitir `model` mapeado dentro de `agent-markdown`, sem afetar gemini/kiro
> Dependências: Wave 1 completa. As 3 MLs tocam arquivos/linguagens distintas —
> executar em paralelo. Independente da Wave 2 (arquivos e branches diferentes
> dentro do mesmo `Render()`/`render()`), pode rodar em paralelo com a Wave 2.

**Mapeamento de modelo (fonte: `cursor.com/docs/subagents`, pesquisado em
2026-08-14 — ver ADR vinculado):**
- `opus` → `claude-opus-5[effort=high]`
- `sonnet` → `composer-2.5[fast=true]`
- qualquer outro valor (ou ausente) → omitir a linha `model:` (deixa o Cursor cair
  no default `inherit`/Auto)

> ⚠️ Sem instância local do Cursor para teste automatizado nesta sessão — a Wave 5
> exige confirmação manual do usuário antes do REQ ir para `Done`.

### ML-3A — Go: reescrita de `model:` no branch `default`/`agent-markdown` quando `targetID == "cursor"`
**Status:** ✅ Concluído
**Arquivos afetados:**
- `internal/integrations/render.go`
- `internal/integrations/render_test.go`
**Ações:**
1. Adicionar função `mapModelCursor(model string) (string, bool)`, mesmo formato de
   `mapModelCodex` (ML-2A), com os valores do mapeamento acima.
2. No branch `default:` de `Render()` (linha ~127), antes do `if !hasIdentity` atual,
   adicionar:
   ```go
   if targetID == "cursor" {
       if mapped, ok := mapModelCursor(model); ok {
           source = rewriteFrontmatterModelLine(source, mapped)
       } else {
           source = removeFrontmatterModelLine(source)
       }
   }
   ```
   (nomes de função ilustrativos — implementar como função auxiliar nova,
   `rewriteFrontmatterModelLine(source []byte, value string) []byte`, análoga a
   `rewriteFrontmatterFields` mas escopada só à chave `model:`; usar
   `removeFrontmatterModelLine` para o caso de omissão). Aplicar **antes** do
   despacho para `hasIdentity`/`rewriteFrontmatterFields`, para que as duas
   transformações componham sem se pisar (uma mexe em `name`/`description`, a
   outra só em `model`).
3. Confirmar explicitamente que `gemini` e `kiro` (mesmo branch `default`, mesma
   representação `agent-markdown`) **não** entram nesse `if` — nenhuma outra
   condição além de `targetID == "cursor"`.
4. Testes em `render_test.go`: casos com `targetID: "cursor"` (esperando `model:`
   reescrito) e `targetID: "gemini"` / `targetID: "kiro"` (esperando output
   byte-a-byte idêntico ao pré-Wave-3 — teste de regressão explícito, ver Wave 4).
**Critérios de aceite:**
- [ ] `go build ./...` sem erros
- [ ] `go test ./internal/integrations/...` verde
- [ ] `.cursor/agents/trackfw-architect.md` gerado por `trackfw init --ai-tools cursor`
      contém `model: claude-opus-5[effort=high]`; `trackfw-backend.md` contém
      `model: composer-2.5[fast=true]`
**Comandos de validação:** `go build ./... && go test ./internal/integrations/...`

### ML-3B — Node.js: mesma lógica em `render.js`
**Status:** ✅ Concluído
**Arquivos afetados:**
- `npm/src/integrations/render.js`
**Ações:** Espelhar ML-3A: `mapModelCursor()`, reescrita/remoção da linha `model:`
condicionada a `target === 'cursor'` dentro do branch equivalente ao `default` do Go,
aplicada antes da lógica de identidade existente. Atualizar/criar teste equivalente.
**Critérios de aceite:**
- [ ] `npm test` verde
- [ ] Output byte-a-byte idêntico ao Go para o mesmo input
**Comandos de validação:** `npm test --workspace=trackfw`

### ML-3C — Python: mesma lógica em `renderers.py`
**Status:** ✅ Concluído
**Arquivos afetados:**
- `pypi/trackfw/integrations/renderers.py`
**Ações:** Espelhar ML-3A: `_map_model_cursor()`, aplicado no branch final (Rota B,
linha ~319) antes de `_rewrite_frontmatter_fields`, condicionado a
`target == "cursor"`. Atualizar/criar teste equivalente.
**Critérios de aceite:**
- [ ] `pytest` verde
- [ ] Output byte-a-byte idêntico ao Go para o mesmo input
**Comandos de validação:** `cd pypi && python -m pytest`

## Wave 4 — Regressão gemini/kiro + paridade
> Dependências: Wave 2 e Wave 3 completas nos 3 CLIs.

### ML-4A — Teste de regressão explícito: gemini/kiro inalterados (3 CLIs)
**Status:** ✅ Concluído (entregue inline durante a Wave 3, não como ML separada)
**Arquivos afetados:**
- `internal/integrations/render_test.go` (`TestRenderSubagentRouteGeminiKiroUnaffectedByCursorMapping`)
- `npm/tests/identity-render.test.js` ("gemini e kiro (mesma representação agent-markdown do cursor) permanecem bit-a-bit inalterados")
- `pypi/tests/test_integrations_identity.py` (`TestCursorModelMapping`, casos gemini/kiro)
**Ações:** Cada agente da Wave 3 (ML-3A/3B/3C) já incluiu, por instrução explícita do
prompt, um teste de regressão comparando o output de `gemini`/`kiro` antes e depois da
mudança — satisfazendo o critério desta ML sem necessidade de trabalho adicional.
**Critérios de aceite:**
- [x] Teste falha se qualquer mudança futura alterar output de gemini/kiro
- [x] Verde nos 3 CLIs
**Comandos de validação:** `go test ./internal/integrations/... && npm test --workspace=trackfw && (cd pypi && python -m pytest)`

### ML-4B — `make quality` completo
**Status:** ✅ Concluído
**Arquivos afetados:** nenhum (validação apenas)
**Ações:** Rodado `make quality` na raiz do repo.
**Critérios de aceite:**
- [x] `make quality` verde — exit code 0, 112 cenários de falsificação (18 gates + 11
      contratos de gerador/validador) e contratos de paridade Go/Node/Python passaram.
**Comandos de validação:** `make quality`

## Wave 5 — Verificação manual e fechamento
> Dependências: Wave 4 completa.

### ML-5A — Confirmação manual da sintaxe Cursor e Codex antes de mover REQ para Done
**Status:** ✅ Concluído
**Arquivos afetados:** nenhum (verificação, não código)
**Ações:**
1. Rodado `trackfw init` (via `trackfw agents install --targets codex|cursor`) em um
   diretório de scratch isolado (`$HOME` sintético, não o do usuário).
2. Sem Cursor/Codex CLI instalados neste ambiente para teste end-to-end — limitação
   registrada desde o ADR original.
3. Output real inspecionado e conferido contra a documentação oficial já citada no
   ADR (`cursor.com/docs/subagents`, guia de `config.toml` do Codex CLI):
   - `.codex/agents/trackfw-architect.toml` → `model = "gpt-5.4"`
   - `.codex/agents/trackfw-backend.toml` → `model = "gpt-5.4-mini"`
   - `.cursor/agents/trackfw-architect.md` → `model: claude-opus-5[effort=high]`
   - `.cursor/agents/trackfw-backend.md` → `model: composer-2.5[fast=true]`
4. Usuário optou explicitamente por **aceitar confirmação documental** (em vez de
   testar localmente ou deixar aberto) via `AskUserQuestion` — decisão registrada
   aqui como evidência de fechamento. Risco residual aceito: a sintaxe pode divergir
   se o Cursor/Codex mudarem o formato aceito após 2026-08-14.
**Critérios de aceite:**
- [x] REQ e ADR movidos para `Done`/`Accepted` após esta confirmação
      (documental, não testada ao vivo — risco residual aceito pelo usuário)
**Comandos de validação:** inspeção manual, sem comando automatizado único
